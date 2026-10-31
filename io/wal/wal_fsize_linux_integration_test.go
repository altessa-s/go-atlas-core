//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package wal_test

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/wal"
)

// TestWALIntegration_PartialWriteIsRolledBack lowers RLIMIT_FSIZE in a child
// process so the kernel accepts only part of a record and then fails the
// write with EFBIG. The WAL must report the error, cut the partial bytes off
// the segment, and write the next record at the offset the failed one would
// have taken, so recovery sees only whole records. The limit is per process,
// so the child keeps it away from the other tests.
func TestWALIntegration_PartialWriteIsRolledBack(t *testing.T) {
	first := []byte("first record")
	last := []byte("written after the limit is lifted")
	if dir, ok := childDir(t); ok {
		runPartialWriteChild(t, dir, first, last)
		return
	}
	t.Parallel()
	dir := privateDir(t)
	out := runChild(t, dir, 0)
	require.Contains(t, out, "partial write rolled back")

	w, recovered, err := wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.Len(t, recovered, 2)
	require.Equal(t, first, recovered[0].Payload)
	require.Equal(t, last, recovered[1].Payload)
	require.Equal(t, encodedSize(len(first)), recovered[1].Offset.Position)
}

func runPartialWriteChild(t *testing.T, dir string, first, last []byte) {
	w, _, err := wal.Open(dir, wal.WithFsyncInterval(time.Hour))
	require.NoError(t, err)
	_, err = w.Append(first)
	require.NoError(t, err)
	segs := segmentFiles(t, dir)
	require.Len(t, segs, 1)
	size := fileSize(t, segs[0])
	require.Equal(t, encodedSize(len(first)), size)

	var limit syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit))
	lowered := limit
	lowered.Cur = uint64(size) + 10 // room for part of the next record only
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lowered))

	_, err = w.Append(bytes.Repeat([]byte("x"), 100))
	require.ErrorIs(t, err, syscall.EFBIG)
	require.Equal(t, size, fileSize(t, segs[0]), "the partial bytes are truncated away")

	// Raising the soft limit back up to the unchanged hard limit is allowed.
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit))
	off, err := w.Append(last)
	require.NoError(t, err)
	require.Equal(t, size, off.Position, "the next record starts where the failed one would have")
	require.NoError(t, w.Close())
	_, err = fmt.Fprintln(os.Stdout, "partial write rolled back")
	require.NoError(t, err)
}
