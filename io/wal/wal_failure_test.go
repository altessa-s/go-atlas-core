// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package wal

import (
	"bytes"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// swapActiveFile installs f as the active segment's file handle and returns
// the previous one, so a test can inject a handle whose writes or syncs fail.
func swapActiveFile(tb testing.TB, w *WAL, f *os.File) *os.File {
	tb.Helper()
	w.wmu.Lock()
	defer w.wmu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	require.NotNil(tb, w.active)
	old := w.active.file
	w.active.file = f
	return old
}

// activePath returns the path of the active segment.
func activePath(tb testing.TB, w *WAL) string {
	tb.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	require.NotNil(tb, w.active)
	return w.active.path
}

// closedFile returns a handle of path that is already closed, so every Write
// and Sync on it fails with os.ErrClosed.
func closedFile(tb testing.TB, path string) *os.File {
	tb.Helper()
	f, err := os.Open(path) //nolint:gosec // test-owned segment path
	require.NoError(tb, err)
	require.NoError(tb, f.Close())
	return f
}

// manualTicker returns a TickerFunc whose ticks the test sends by hand. The
// channel is unbuffered: a second send returns only after the background
// fsync started by the first one has finished.
func manualTicker() (chan time.Time, Option) {
	ticks := make(chan time.Time)
	return ticks, WithFsyncTicker(func(time.Duration) (<-chan time.Time, func()) {
		return ticks, func() {}
	})
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of a logger
// used by the background fsync goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func recoveredPayloads(records []Record) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, string(r.Payload))
	}
	return out
}

func TestOptions_IgnoreInvalidValues(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	tests := []struct {
		name     string
		opts     []Option
		interval time.Duration
		logger   *slog.Logger
	}{
		{name: "zero interval", opts: []Option{WithFsyncInterval(0)}, interval: DefaultFsyncInterval},
		{name: "negative interval", opts: []Option{WithFsyncInterval(-time.Second)}, interval: DefaultFsyncInterval},
		{name: "positive interval", opts: []Option{WithFsyncInterval(time.Second)}, interval: time.Second},
		{name: "nil ticker", opts: []Option{WithFsyncTicker(nil)}, interval: DefaultFsyncInterval},
		{name: "nil logger", opts: []Option{WithLogger(nil)}, interval: DefaultFsyncInterval},
		{name: "logger", opts: []Option{WithLogger(logger)}, interval: DefaultFsyncInterval, logger: logger},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := newOptions(tc.opts...)
			require.Equal(t, tc.interval, o.fsyncInterval)
			require.NotNil(t, o.fsyncTicker, "a nil ticker must not replace the default")
			require.NotNil(t, o.logger, "a nil logger must not replace the default")
			if tc.logger != nil {
				require.Same(t, tc.logger, o.logger)
			}
		})
	}
}

// TestWAL_Append_WriteFailureLeavesSegmentConsistent fails a group-commit
// write and checks that the failed record is reported to its Append, is not
// counted, and does not shift the offset of the next record or survive
// recovery.
func TestWAL_Append_WriteFailureLeavesSegmentConsistent(t *testing.T) {
	t.Parallel()
	w, dir := newTestWAL(t, WithFsyncInterval(time.Hour))

	first, err := w.Append([]byte("first"))
	require.NoError(t, err)
	before := w.Stats()

	readOnly, err := os.Open(activePath(t, w)) //nolint:gosec // test-owned segment path
	require.NoError(t, err)
	writable := swapActiveFile(t, w, readOnly)

	off, err := w.Append([]byte("second"))
	require.Error(t, err)
	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr, "the write error is wrapped, not replaced")
	require.Equal(t, "write", pathErr.Op)
	require.True(t, off.IsZero(), "a failed Append returns no offset")
	require.Equal(t, before, w.Stats(), "a failed write is not counted")

	require.Same(t, readOnly, swapActiveFile(t, w, writable))
	require.NoError(t, readOnly.Close())

	third, err := w.Append([]byte("third"))
	require.NoError(t, err)
	require.Equal(t, first.SegmentID, third.SegmentID)
	require.Equal(t, first.Position+recordHeaderSize+int64(len("first")), third.Position,
		"the failed record must not advance the segment position")
	require.NoError(t, w.Close())

	w2, recovered, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w2.Close() })
	require.Equal(t, []string{"first", "third"}, recoveredPayloads(recovered))
}

// TestWAL_SegmentRollFailureIsReported fails the final sync of the segment
// being sealed and checks that the Append needing the roll gets the error
// and that a later roll succeeds once the handle is healthy again.
func TestWAL_SegmentRollFailureIsReported(t *testing.T) {
	t.Parallel()
	w, dir := newTestWAL(t, WithMaxSegmentBytes(16), WithFsyncInterval(time.Hour))

	first, err := w.Append([]byte("12345678")) // fills the 16-byte segment
	require.NoError(t, err)

	healthy := swapActiveFile(t, w, closedFile(t, activePath(t, w)))
	_, err = w.Append([]byte("rolls"))
	require.ErrorIs(t, err, os.ErrClosed, "the failed seal sync is returned to the Append that needed the roll")
	require.Equal(t, 1, w.Stats().Segments, "a failed seal must not open a new segment")

	swapActiveFile(t, w, healthy)
	second, err := w.Append([]byte("rolls"))
	require.NoError(t, err)
	require.Greater(t, second.SegmentID, first.SegmentID)
	require.Equal(t, int64(0), second.Position)
	require.NoError(t, w.Close())

	w2, recovered, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w2.Close() })
	require.Equal(t, []string{"12345678", "rolls"}, recoveredPayloads(recovered))
}

// TestWAL_BackgroundFsyncFailureIsReported checks that a failing background
// fsync of the active segment is exposed by Faulted and logged, that Sync
// reports the same failure, and that a later successful fsync clears it.
func TestWAL_BackgroundFsyncFailureIsReported(t *testing.T) {
	t.Parallel()
	var logs syncBuffer
	ticks, ticker := manualTicker()
	w, _ := newTestWAL(t, ticker, WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	_, err := w.Append([]byte("payload"))
	require.NoError(t, err)

	healthy := swapActiveFile(t, w, closedFile(t, activePath(t, w)))
	ticks <- time.Time{}
	ticks <- time.Time{}
	require.ErrorIs(t, w.Faulted(), os.ErrClosed)
	require.Contains(t, logs.String(), "background fsync failed")
	require.ErrorIs(t, w.Sync(), os.ErrClosed)

	swapActiveFile(t, w, healthy)
	ticks <- time.Time{}
	ticks <- time.Time{}
	require.NoError(t, w.Faulted(), "a successful fsync clears the fault")
	require.NoError(t, w.Close())
}

// TestWAL_Close_JoinsFinalSyncAndCloseErrors checks that Close reports both
// a failed final sync and a failed final close of the active segment.
func TestWAL_Close_JoinsFinalSyncAndCloseErrors(t *testing.T) {
	t.Parallel()
	w, _ := newTestWAL(t, WithFsyncInterval(time.Hour))
	_, err := w.Append([]byte("payload"))
	require.NoError(t, err)

	healthy := swapActiveFile(t, w, closedFile(t, activePath(t, w)))
	t.Cleanup(func() { _ = healthy.Close() })

	err = w.Close()
	require.ErrorIs(t, err, os.ErrClosed)
	var joined interface{ Unwrap() []error }
	require.ErrorAs(t, err, &joined)
	require.Len(t, joined.Unwrap(), 2, "both the final sync and the final close failed")
	require.NoError(t, w.Close(), "a second Close is a no-op")
}

// TestWAL_RetiredActiveFile checks the state the WAL is left in when the
// active segment's handle was retired after a failed roll-back: Append
// returns ErrClosed while Sync, the background fsync and Close do nothing.
func TestWAL_RetiredActiveFile(t *testing.T) {
	t.Parallel()
	ticks, ticker := manualTicker()
	w, dir := newTestWAL(t, ticker)
	_, err := w.Append([]byte("kept"))
	require.NoError(t, err)

	f := swapActiveFile(t, w, nil)
	require.NoError(t, f.Close())

	_, err = w.Append([]byte("rejected"))
	require.ErrorIs(t, err, ErrClosed)
	require.NoError(t, w.Sync())
	ticks <- time.Time{}
	ticks <- time.Time{}
	require.NoError(t, w.Faulted())
	require.NoError(t, w.Close())

	w2, recovered, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w2.Close() })
	require.Equal(t, []string{"kept"}, recoveredPayloads(recovered))
}

// TestWAL_Close_ReportsRemoveFailure checks that Close returns the error of
// removing a fully acked active segment instead of dropping it.
func TestWAL_Close_ReportsRemoveFailure(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX directory permissions enforced for the current user")
	}
	w, dir := newTestWAL(t, WithFsyncInterval(time.Hour))
	off, err := w.Append([]byte("acked"))
	require.NoError(t, err)
	w.Ack(off)

	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err = w.Close()
	require.ErrorIs(t, err, fs.ErrPermission)

	require.NoError(t, os.Chmod(dir, 0o700))
	matches, err := filepath.Glob(filepath.Join(dir, "*"+fileSuffix))
	require.NoError(t, err)
	require.Len(t, matches, 1, "the segment that could not be removed stays on disk")
}
