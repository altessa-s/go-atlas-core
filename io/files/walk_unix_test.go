// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build unix

package files_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/files"
)

// TestWalk_FileTypeOther exercises the FileTypeOther classification
// (fileTypeFromMode's default case) using a FIFO, which is neither a
// regular file, a directory, nor a symlink. syscall.Mkfifo is unix-only,
// hence the build tag.
func TestWalk_FileTypeOther(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fifoPath := filepath.Join(root, "pipe")
	require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))

	var gotType files.FileType
	var found bool
	for entry, err := range files.Walk(root) {
		require.NoError(t, err)
		if entry.Name() == "pipe" {
			found = true
			gotType = entry.Type()
		}
	}
	require.True(t, found, "expected to see the FIFO entry")
	require.Equal(t, files.FileTypeOther, gotType)

	// FileTypeOther must also be selectable as an explicit filter.
	names, _ := collect(t, root, files.WithFileTypes(files.FileTypeOther))
	require.Equal(t, []string{"pipe"}, names)
}
