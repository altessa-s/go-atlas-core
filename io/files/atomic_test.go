// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package files_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/files"
)

func TestWriteFileAtomic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := filepath.Join(dir, "target")
	for _, tc := range []struct {
		data string
		perm fs.FileMode
	}{{"first contents", 0o600}, {"new", 0o640}} {
		require.NoError(t, files.WriteFileAtomic(path, []byte(tc.data), tc.perm))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, tc.data, string(got))

		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, tc.perm, info.Mode().Perm())
		}
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
func TestWriteFileAtomicRenameFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(path, 0o700))
	err := files.WriteFileAtomic(path, nil, 0o600)

	var rename *os.LinkError
	require.ErrorAs(t, err, &rename)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, entries[0].IsDir())
	require.Equal(t, "target", entries[0].Name())
}
