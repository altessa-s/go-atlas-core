// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build unix

package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/files"
)

func TestWalk_EarlyStopPin(t *testing.T) {
	t.Parallel()

	t.Run("stopping on a broken symlink error ends the walk", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "a-broken")))
		require.NoError(t, os.WriteFile(filepath.Join(root, "b-file"), nil, 0o600))

		var seen []string
		for e, err := range files.Walk(root, files.WithRecursive(), files.WithFollowSymlinks()) {
			seen = append(seen, filepath.Base(e.Path))
			if err != nil {
				break
			}
		}
		require.Equal(t, []string{"a-broken", "a-broken"}, seen, "the link, then its stat error; b-file never visited")
	})

	t.Run("stopping inside a subdirectory ends the walk", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, "a-dir"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "a-dir", "inner"), nil, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(root, "b-file"), nil, 0o600))

		var seen []string
		for e, err := range files.Walk(root, files.WithRecursive()) {
			require.NoError(t, err)
			seen = append(seen, filepath.Base(e.Path))
			if filepath.Base(e.Path) == "inner" {
				break
			}
		}
		require.Equal(t, []string{"a-dir", "inner"}, seen)
	})
}
