// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package spool_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/spool"
)

func TestWithTempDir(t *testing.T) {
	t.Parallel()

	for _, tee := range []bool{false, true} {
		dir := t.TempDir()
		opts := []spool.Option{spool.WithMemThreshold(4), spool.WithTempDir(dir)}
		if tee {
			opts = append(opts, spool.WithTee(io.Discard))
		}
		sp, err := spool.New(strings.NewReader("spills to disk"), opts...)
		require.NoError(t, err)

		files, err := filepath.Glob(filepath.Join(dir, "spool-*"))
		require.NoError(t, err)
		require.Len(t, files, 1, "the spill file lives in the configured directory (tee=%v)", tee)

		var got bytes.Buffer
		_, err = io.Copy(&got, sp)
		require.NoError(t, err)
		require.Equal(t, "spills to disk", got.String())
		require.NoError(t, sp.Close())

		_, err = os.Stat(files[0])
		require.ErrorIs(t, err, os.ErrNotExist, "Close removes the spill file")
	}
}

// TestWithTempDirValues checks how WithTempDir normalizes its argument: the
// value is trimmed, and a blank string or a nil pointer leaves the default
// directory for temporary files in place.
func TestWithTempDirValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blank := "   "
	tests := []struct {
		name    string
		opt     spool.Option
		wantDir string
	}{
		{name: "padded string", opt: spool.WithTempDir(" \t" + dir + "\n"), wantDir: dir},
		{name: "pointer", opt: spool.WithTempDir(&dir), wantDir: dir},
		{name: "blank string", opt: spool.WithTempDir(blank), wantDir: os.TempDir()},
		{name: "blank pointer", opt: spool.WithTempDir(&blank), wantDir: os.TempDir()},
		{name: "nil pointer", opt: spool.WithTempDir((*string)(nil)), wantDir: os.TempDir()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sp, err := spool.New(strings.NewReader("spills to disk"), spool.WithMemThreshold(4), tc.opt)
			require.NoError(t, err)
			t.Cleanup(func() { _ = sp.Close() })

			f, ok := sp.ReadSeeker.(*os.File)
			require.True(t, ok, "the content is spilled to a file")
			require.Equal(t, filepath.Clean(tc.wantDir), filepath.Dir(f.Name()))
		})
	}
}

func TestWithTempDirMissing(t *testing.T) {
	t.Parallel()

	_, err := spool.New(strings.NewReader("spills to disk"),
		spool.WithMemThreshold(4), spool.WithTempDir(filepath.Join(t.TempDir(), "absent")))
	require.ErrorIs(t, err, os.ErrNotExist)
}
