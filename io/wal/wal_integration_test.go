//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that run the WAL against real segment files across
// process restarts: reopen cycles with acks, torn tails cut into the files on
// disk, a process that exits without Close, and the real background fsync
// racing segment rotation.

package wal_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/wal"
)

// integrationChildEnv names the test a re-executed binary must run as a
// child; integrationDirEnv carries the WAL directory it works in.
const (
	integrationChildEnv = "ATLAS_WAL_INTEGRATION_CHILD"
	integrationDirEnv   = "ATLAS_WAL_INTEGRATION_DIR"
)

// childDir reports whether this process is the child half of t and returns
// the WAL directory the parent handed it.
func childDir(t *testing.T) (string, bool) {
	t.Helper()
	if os.Getenv(integrationChildEnv) != t.Name() {
		return "", false
	}
	return os.Getenv(integrationDirEnv), true
}

// runChild re-executes the test binary to run only t in a child process
// working on dir and returns its combined output. The child is expected to
// exit with wantCode.
func runChild(t *testing.T, dir string, wantCode int) string {
	t.Helper()
	//nolint:gosec // re-executes this test binary
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), integrationChildEnv+"="+t.Name(), integrationDirEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	if wantCode == 0 {
		require.NoError(t, err, "child output:\n%s", out)
	} else {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "child output:\n%s", out)
		require.Equal(t, wantCode, exitErr.ExitCode(), "child output:\n%s", out)
	}
	return string(out)
}

// privateDir returns a fresh WAL directory with the permissions Open requires.
func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	return dir
}

// segmentFiles returns the segment files in dir in id order.
func segmentFiles(t *testing.T, dir string) []string {
	t.Helper()
	segs, err := filepath.Glob(filepath.Join(dir, "*.wal"))
	require.NoError(t, err)
	slices.Sort(segs)
	return segs
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

// encodedSize is the on-disk size of a record with an n-byte payload.
func encodedSize(n int) int64 { return 8 + int64(n) }

// TestWALIntegration_RestartCycles reopens the same directory several times.
// Each session must recover exactly the records left by the previous ones at
// their original offsets, acking a whole segment must delete its file, and
// new segments must never reuse the id of an existing one.
func TestWALIntegration_RestartCycles(t *testing.T) {
	t.Parallel()
	dir := privateDir(t)
	opts := []wal.Option{wal.WithMaxSegmentBytes(1 << 10), wal.WithFsyncInterval(time.Hour)}

	var (
		want   []wal.Record
		nextID uint64 = 1
	)
	for session := range 4 {
		w, recovered, err := wal.Open(dir, opts...)
		require.NoError(t, err, "session %d", session)
		require.Equal(t, want, recovered, "session %d: recovery must return the surviving records unchanged", session)

		if len(want) > 0 {
			oldest := want[0].Offset.SegmentID
			oldestPath := segmentFiles(t, dir)[0]
			for _, r := range want {
				if r.Offset.SegmentID == oldest {
					w.Ack(r.Offset)
				}
			}
			_, err = os.Stat(oldestPath)
			require.ErrorIs(t, err, fs.ErrNotExist, "session %d: a fully acked segment is deleted", session)
			want = slices.DeleteFunc(want, func(r wal.Record) bool { return r.Offset.SegmentID == oldest })
		}

		maxSeen := uint64(0)
		for _, r := range recovered {
			maxSeen = max(maxSeen, r.Offset.SegmentID)
		}
		for i := range 50 {
			payload := recordPayload(nextID)
			nextID++
			off, err := w.Append(payload)
			require.NoError(t, err, "session %d record %d", session, i)
			require.Greater(t, off.SegmentID, maxSeen, "session %d: new records go to new segments", session)
			want = append(want, wal.Record{Offset: off, Payload: payload})
		}
		require.NoError(t, w.Close(), "session %d", session)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
		for _, seg := range segmentFiles(t, dir) {
			info, err := os.Stat(seg)
			require.NoError(t, err)
			require.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), seg)
		}
	}
}

// TestWALIntegration_TornTailIsTruncatedOnDisk cuts the last record of a
// segment in half, as a crash in the middle of a write would. Reopening must
// drop the torn record, shrink the file to the valid prefix, and place the
// next record right after it, so a second restart sees no garbage in between.
func TestWALIntegration_TornTailIsTruncatedOnDisk(t *testing.T) {
	t.Parallel()
	dir := privateDir(t)

	w, _, err := wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	var validSize int64
	const records = 20
	for id := uint64(1); id <= records; id++ {
		payload := recordPayload(id)
		_, err := w.Append(payload)
		require.NoError(t, err)
		if id < records {
			validSize += encodedSize(len(payload))
		}
	}
	require.NoError(t, w.Close())

	segs := segmentFiles(t, dir)
	require.Len(t, segs, 1)
	require.NoError(t, os.Truncate(segs[0], fileSize(t, segs[0])-3))

	w, recovered, err := wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	require.Len(t, recovered, records-1, "the torn record is dropped")
	require.Equal(t, validSize, fileSize(t, segs[0]), "recovery truncates the file to its valid prefix")
	off, err := w.Append([]byte("after the crash"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	w, recovered, err = wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.Len(t, recovered, records)
	for i, r := range recovered[:records-1] {
		id, ok := recordID(r.Payload)
		require.True(t, ok)
		require.Equal(t, uint64(i+1), id)
	}
	require.Equal(t, off, recovered[records-1].Offset)
	require.Equal(t, "after the crash", string(recovered[records-1].Payload))
}

// TestWALIntegration_GarbageSegmentIsRemoved plants a segment file that holds
// no valid record. Open must delete it and keep working in the directory.
func TestWALIntegration_GarbageSegmentIsRemoved(t *testing.T) {
	t.Parallel()
	dir := privateDir(t)
	garbage := filepath.Join(dir, fmt.Sprintf("%020d.wal", 99))
	require.NoError(t, os.WriteFile(garbage, bytes.Repeat([]byte{0xff}, 64), 0o600))

	w, recovered, err := wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	require.Empty(t, recovered)
	_, err = os.Stat(garbage)
	require.ErrorIs(t, err, fs.ErrNotExist, "a segment without a valid record is deleted")

	_, err = w.Append([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	w, recovered, err = wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.Equal(t, []string{"payload"}, payloads(recovered))
}

func payloads(records []wal.Record) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, string(r.Payload))
	}
	return out
}

// TestWALIntegration_ExitWithoutClose appends from a child process that exits
// without Close or Sync. Every Append that returned has been written to the
// file, so a process exit must not lose any of them.
func TestWALIntegration_ExitWithoutClose(t *testing.T) {
	const records = 300
	if dir, ok := childDir(t); ok {
		w, recovered, err := wal.Open(dir, wal.WithMaxSegmentBytes(4<<10), wal.WithFsyncInterval(time.Hour))
		require.NoError(t, err)
		require.Empty(t, recovered)
		for id := uint64(1); id <= records; id++ {
			_, err := w.Append(recordPayload(id))
			require.NoError(t, err)
		}
		os.Exit(3) // no Close, no Sync
	}
	t.Parallel()
	dir := privateDir(t)
	runChild(t, dir, 3)

	require.Greater(t, len(segmentFiles(t, dir)), 1, "the child must have rolled segments")
	w, recovered, err := wal.Open(dir, wal.WithMaxSegmentBytes(4<<10), wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.Len(t, recovered, records)
	for i, r := range recovered {
		id, ok := recordID(r.Payload)
		require.True(t, ok, "corrupt record at %+v", r.Offset)
		require.Equal(t, uint64(i+1), id)
	}
}

// TestWALIntegration_BackgroundFsyncDuringRotation runs the real background
// fsync on a short interval while concurrent appends roll small segments.
// The fsync can catch a segment just as it is sealed and closed; that race
// is benign and must never surface as a fault.
func TestWALIntegration_BackgroundFsyncDuringRotation(t *testing.T) {
	t.Parallel()
	dir := privateDir(t)
	opts := []wal.Option{wal.WithMaxSegmentBytes(512), wal.WithFsyncInterval(100 * time.Microsecond)}
	w, _, err := wal.Open(dir, opts...)
	require.NoError(t, err)

	const writers, perWriter = 4, 400
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		faults   []error
		appended = make(map[string]struct{}, writers*perWriter)
	)
	for g := range writers {
		wg.Go(func() {
			for i := range perWriter {
				payload := fmt.Sprintf("writer-%d-record-%d", g, i)
				_, err := w.Append([]byte(payload))
				fault := w.Faulted()
				mu.Lock()
				if err != nil || fault != nil {
					faults = append(faults, err, fault)
				}
				appended[payload] = struct{}{}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	require.Empty(t, slices.DeleteFunc(faults, func(err error) bool { return err == nil }))
	require.NoError(t, w.Faulted())
	require.NoError(t, w.Close())

	w, recovered, err := wal.Open(dir, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.Len(t, recovered, writers*perWriter)
	for _, r := range recovered {
		_, ok := appended[string(r.Payload)]
		require.True(t, ok, "unexpected record %q", r.Payload)
	}
}
