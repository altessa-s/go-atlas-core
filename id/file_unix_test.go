// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build unix

package id

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"golang.org/x/sys/unix"
)

func TestFile_RejectsSymlink(t *testing.T) {
	t.Parallel()

	for _, exists := range []bool{true, false} {
		name := "dangling"
		if exists {
			name = "existing target"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, ".sid")

			target := filepath.Join(dir, "target")
			if exists {
				require.NoError(t, os.WriteFile(target, []byte("target-id"), 0o600))
			}

			require.NoError(t, os.Symlink(target, path))

			p := MustNewFile(path)
			require.Empty(t, p.ID())
			require.Error(t, p.Error())

			info, err := os.Lstat(path)
			require.NoError(t, err)
			require.Equal(t, os.ModeSymlink, info.Mode().Type())

			if !exists {
				_, err := os.Stat(target)
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestFile_RejectsFIFOWithoutBlocking(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".sid")
	require.NoError(t, unix.Mkfifo(path, 0o600))

	p := MustNewFile(path)

	result := make(chan error, 1)
	go func() { result <- p.Error() }()

	select {
	case err := <-result:
		require.ErrorIs(t, err, errNonRegularIDFile)
		require.Empty(t, p.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("opening the FIFO blocked without a writer")
	}
}
