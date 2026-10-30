// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build unix

package files_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/files"
)

func TestOpenRegular(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regular := filepath.Join(dir, "a.yaml")
	require.NoError(t, os.WriteFile(regular, []byte("x: 1\n"), 0o600))
	fifo := filepath.Join(dir, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))

	f, err := files.OpenRegular(regular)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = files.OpenRegular(dir)
	require.ErrorIs(t, err, files.ErrNotRegular)

	done := make(chan error, 1)
	go func() {
		_, err := files.OpenRegular(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, files.ErrNotRegular)
	case <-time.After(5 * time.Second):
		t.Fatal("opening a FIFO without a writer blocked")
	}

	_, err = files.OpenRegular(filepath.Join(dir, "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestReadRegular(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regular := filepath.Join(dir, "a.yaml")
	require.NoError(t, os.WriteFile(regular, []byte("x: 1\n"), 0o600))
	fifo := filepath.Join(dir, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))

	got, err := files.ReadRegular(regular)
	require.NoError(t, err)
	require.Equal(t, "x: 1\n", string(got))

	_, err = files.ReadRegular(filepath.Join(dir, "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)

	done := make(chan error, 1)
	go func() {
		_, err := files.ReadRegular(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, files.ErrNotRegular)
	case <-time.After(5 * time.Second):
		t.Fatal("reading a FIFO without a writer blocked")
	}
}

func TestOpenRegularIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("secret: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x: 1\n"), 0o600))
	require.NoError(t, os.Symlink("a.yaml", filepath.Join(dir, "inside.yaml")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape.yaml")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "fifo.yaml"), 0o600))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	for _, name := range []string{"a.yaml", "inside.yaml"} {
		f, err := files.OpenRegularIn(root, name)
		require.NoError(t, err, name)
		require.NoError(t, f.Close())
	}

	_, err = files.OpenRegularIn(root, "escape.yaml")
	require.Error(t, err, "a symlink leading outside the root was opened")

	_, err = files.OpenRegularIn(root, "fifo.yaml")
	require.ErrorIs(t, err, files.ErrNotRegular)
}

// OpenRootDir refuses a FIFO at once instead of blocking on it, and opens a
// directory, with or without a trailing separator.
func TestOpenRootDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, path := range []string{dir, dir + string(filepath.Separator)} {
		root, err := files.OpenRootDir(path)
		require.NoError(t, err)
		require.NoError(t, root.Close())
	}

	_, err := files.OpenRootDir("")
	require.Error(t, err, "an empty path must not open the filesystem root")

	fifo := filepath.Join(dir, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	done := make(chan error, 1)
	go func() {
		root, err := files.OpenRootDir(fifo)
		if err == nil {
			_ = root.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("opening a FIFO as a root blocked")
	}
}
