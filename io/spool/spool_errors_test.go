// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package spool_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/spool"
)

var errSource = errors.New("source failed")

// failingReader yields data and then fails with err.
func failingReader(data string, err error) io.Reader {
	return io.MultiReader(strings.NewReader(data), errReader{err: err})
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// failingWriter accepts limit bytes and then fails with err.
type failingWriter struct {
	limit int
	err   error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit {
		n := w.limit
		w.limit = 0
		return n, w.err
	}
	w.limit -= len(p)
	return len(p), nil
}

// requireNoSpillFiles checks that dir holds no leftover spill file.
func requireNoSpillFiles(tb testing.TB, dir string) {
	tb.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "spool-*"))
	require.NoError(tb, err)
	require.Empty(tb, files, "a failed New must remove its spill file")
}

func TestNew_SourceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		maxSize int64
		tee     bool
	}{
		{name: "before the threshold", data: "abc"},
		{name: "before the threshold with tee", data: "abc", tee: true},
		{name: "after spilling", data: strings.Repeat("x", 64)},
		{name: "after spilling with tee", data: strings.Repeat("x", 64), tee: true},
		{name: "after spilling under a cap", data: strings.Repeat("x", 64), maxSize: 1 << 10},
		{name: "after spilling under a cap with tee", data: strings.Repeat("x", 64), maxSize: 1 << 10, tee: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			opts := []spool.Option{spool.WithMemThreshold(16), spool.WithTempDir(dir), spool.WithMaxBytes(tc.maxSize)}
			if tc.tee {
				opts = append(opts, spool.WithTee(io.Discard))
			}
			sp, err := spool.New(failingReader(tc.data, errSource), opts...)
			require.ErrorIs(t, err, errSource)
			require.Nil(t, sp)
			requireNoSpillFiles(t, dir)
		})
	}
}

func TestNew_TeeErrors(t *testing.T) {
	t.Parallel()

	errTee := errors.New("tee failed")
	tests := []struct {
		name  string
		limit int
	}{
		{name: "while reading the head", limit: 4},
		{name: "while spilling", limit: 40},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			sp, err := spool.New(strings.NewReader(strings.Repeat("y", 4<<10)),
				spool.WithMemThreshold(16), spool.WithTempDir(dir),
				spool.WithTee(&failingWriter{limit: tc.limit, err: errTee}))
			require.ErrorIs(t, err, errTee)
			require.Nil(t, sp)
			requireNoSpillFiles(t, dir)
		})
	}
}

func TestNew_TeeMaxBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		threshold int64
		max       int64
	}{
		{name: "head over the cap", threshold: 1 << 10, max: 8},
		{name: "spill over the cap", threshold: 4, max: 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			sp, err := spool.New(strings.NewReader("more than eight bytes"),
				spool.WithMemThreshold(tc.threshold), spool.WithMaxBytes(tc.max),
				spool.WithTempDir(dir), spool.WithTee(io.Discard))
			require.ErrorIs(t, err, spool.ErrTooLarge)
			require.Nil(t, sp)
			requireNoSpillFiles(t, dir)
		})
	}
}

func TestNew_TeeTempDirMissing(t *testing.T) {
	t.Parallel()

	_, err := spool.New(strings.NewReader("spills to disk"),
		spool.WithMemThreshold(4), spool.WithTee(io.Discard),
		spool.WithTempDir(filepath.Join(t.TempDir(), "absent")))
	require.ErrorIs(t, err, os.ErrNotExist)
}
