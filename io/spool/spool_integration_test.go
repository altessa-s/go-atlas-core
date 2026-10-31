//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that spill real content to a real temp directory with the
// default memory threshold.

package spool_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/spool"
)

// randomContent returns n deterministic pseudo-random bytes.
func randomContent(n int, seed uint64) []byte {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // deterministic test data
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

func spillFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "spool-*"))
	require.NoError(t, err)
	return files
}

// TestSpoolIntegration_SpillsPastDefaultThreshold spools content just over the
// default memory threshold. It must land in a private file of the configured
// directory with exactly the source bytes, read back from any offset, and
// disappear on Close.
func TestSpoolIntegration_SpillsPastDefaultThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := randomContent(int(spool.DefaultMemThreshold)+1, 1)
	digest := sha256.New()

	sp, err := spool.New(bytes.NewReader(content), spool.WithTempDir(dir), spool.WithTee(digest))
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), sp.Size())
	want := sha256.Sum256(content)
	require.Equal(t, want[:], digest.Sum(nil), "the tee sees every byte once")

	files := spillFiles(t, dir)
	require.Len(t, files, 1)
	info, err := os.Stat(files[0])
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), info.Size())
	if runtime.GOOS != "windows" {
		require.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), "the spill file is private")
	}

	for _, off := range []int64{int64(len(content)) - 1, 0, spool.DefaultMemThreshold / 2, spool.DefaultMemThreshold} {
		_, err := sp.Seek(off, io.SeekStart)
		require.NoError(t, err)
		got, err := io.ReadAll(sp)
		require.NoError(t, err)
		require.Equal(t, content[off:], got, "read back from offset %d", off)
	}

	require.NoError(t, sp.Close())
	require.Empty(t, spillFiles(t, dir), "Close removes the spill file")
	require.NoError(t, sp.Close())
}

// TestSpoolIntegration_ConcurrentSpoolsShareDir spools concurrently into one
// directory: each spool gets its own file, reads back its own content, and
// the directory is empty once all are closed.
func TestSpoolIntegration_ConcurrentSpoolsShareDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const spools = 8

	sps := make([]*spool.Spool, spools)
	contents := make([][]byte, spools)
	var wg sync.WaitGroup
	errs := make([]error, spools)
	for i := range spools {
		contents[i] = randomContent(64<<10+i, uint64(i))
		wg.Go(func() {
			sps[i], errs[i] = spool.New(bytes.NewReader(contents[i]), spool.WithMemThreshold(4<<10), spool.WithTempDir(dir))
		})
	}
	wg.Wait()
	for i := range spools {
		require.NoError(t, errs[i])
	}
	require.Len(t, spillFiles(t, dir), spools)

	for i, sp := range sps {
		got, err := io.ReadAll(sp)
		require.NoError(t, err)
		require.Equal(t, contents[i], got)
		require.NoError(t, sp.Close())
	}
	require.Empty(t, spillFiles(t, dir))
}

// TestSpoolIntegration_UnwritableDir spools into a directory the process may
// not create files in. New must fail with a permission error, and content that
// fits in memory must not touch the directory at all.
func TestSpoolIntegration_UnwritableDir(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX directory permissions enforced for the current user")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := spool.New(bytes.NewReader(randomContent(1<<10, 7)), spool.WithMemThreshold(16), spool.WithTempDir(dir))
	require.ErrorIs(t, err, fs.ErrPermission)

	sp, err := spool.New(bytes.NewReader([]byte("small")), spool.WithMemThreshold(16), spool.WithTempDir(dir))
	require.NoError(t, err, "in-memory content never needs the directory")
	require.NoError(t, sp.Close())
}

// TestSpoolIntegration_OverCapLeavesNoFile spools a source that crosses the
// cap after spilling; the partially written file must be removed.
func TestSpoolIntegration_OverCapLeavesNoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := spool.New(bytes.NewReader(randomContent(256<<10, 3)),
		spool.WithMemThreshold(4<<10), spool.WithMaxBytes(128<<10), spool.WithTempDir(dir))
	require.ErrorIs(t, err, spool.ErrTooLarge)
	require.Empty(t, spillFiles(t, dir))
}
