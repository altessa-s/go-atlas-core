//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that walk and open a real directory tree with hidden
// entries, symlink loops, dangling links, links escaping the root and
// entries the process may not read.

package files_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/files"
)

// realTree builds this layout under a fresh temp directory and returns the
// walk root:
//
//	outside/secret.txt
//	root/a.txt
//	root/.hidden.txt
//	root/.hiddendir/inner.txt
//	root/sub/b.txt
//	root/sub/deep/c.txt
//	root/loop     -> .            (cycle back to root)
//	root/self     -> self         (a link to itself)
//	root/dangling -> missing
//	root/outside  -> ../outside   (escapes the root)
//	root/link.txt -> sub/b.txt
func realTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, dir := range []string{"outside", "root/.hiddendir", "root/sub/deep"} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, dir), 0o700))
	}
	for _, file := range []string{"outside/secret.txt", "root/a.txt", "root/.hidden.txt", "root/.hiddendir/inner.txt",
		"root/sub/b.txt", "root/sub/deep/c.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(base, file), []byte(file), 0o600))
	}
	links := map[string]string{"loop": ".", "self": "self", "dangling": "missing", "outside": "../outside", "link.txt": "sub/b.txt"}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	return root
}

// walkResult collects the paths (relative to root) a walk yields and the
// errors it yields, keyed by relative path.
type walkResult struct {
	paths []string
	errs  map[string]error
}

func walkTree(t *testing.T, root string, opts ...files.WalkOption) walkResult {
	t.Helper()
	res := walkResult{errs: make(map[string]error)}
	for entry, err := range files.Walk(root, opts...) {
		rel := ""
		if entry.Path != "" {
			var relErr error
			rel, relErr = filepath.Rel(root, entry.Path)
			require.NoError(t, relErr)
			rel = filepath.ToSlash(rel)
		}
		if err != nil {
			res.errs[rel] = err
			continue
		}
		res.paths = append(res.paths, rel)
	}
	slices.Sort(res.paths)
	return res
}

func TestWalkIntegration_RecursiveWithoutFollowingSymlinks(t *testing.T) {
	t.Parallel()
	root := realTree(t)

	got := walkTree(t, root, files.WithRecursive(), files.WithSkipHidden())
	require.Empty(t, got.errs, "links are listed, not resolved, so a broken one is no error")
	require.Equal(t, []string{
		"a.txt", "dangling", "link.txt", "loop", "outside", "self",
		"sub", "sub/b.txt", "sub/deep", "sub/deep/c.txt",
	}, got.paths)

	got = walkTree(t, root, files.WithRecursive())
	require.Contains(t, got.paths, ".hidden.txt")
	require.Contains(t, got.paths, ".hiddendir/inner.txt", "without WithSkipHidden hidden directories are descended")

	got = walkTree(t, root, files.WithRecursive(), files.WithFileTypes(files.FileTypeSymlink))
	require.Equal(t, []string{"dangling", "link.txt", "loop", "outside", "self"}, got.paths)
}

// TestWalkIntegration_FollowSymlinks follows links with a depth bound: the
// cycle through loop ends at the bound, the link escaping the root is
// descended, and the dangling and self-referencing links surface as errors
// without stopping the walk.
func TestWalkIntegration_FollowSymlinks(t *testing.T) {
	t.Parallel()
	root := realTree(t)

	got := walkTree(t, root, files.WithFollowSymlinks(), files.WithMaxDepth(2), files.WithSkipHidden())
	require.Contains(t, got.paths, "outside/secret.txt", "a followed link may lead outside the root")
	require.Contains(t, got.paths, "loop/sub/b.txt")
	require.Contains(t, got.paths, "loop/loop/a.txt")
	require.Contains(t, got.paths, "sub/deep/c.txt")
	for _, p := range got.paths {
		require.LessOrEqual(t, strings.Count(p, "/"), 2, "%s is deeper than the bound", p)
	}

	require.Contains(t, got.errs, "dangling")
	require.ErrorIs(t, got.errs["dangling"], fs.ErrNotExist)
	require.Contains(t, got.errs, "self", "a self-referencing link cannot be resolved")
	require.Contains(t, got.errs, "loop/dangling", "the walk goes on after a broken link")
	require.NotContains(t, got.paths, "loop/loop/loop/a.txt")
}

// TestWalkIntegration_UnreadableEntries walks a tree with a directory and a
// file the process may not read. The directory is reported as an error and
// the walk continues with its siblings; the file is listed, but reading it
// is refused.
func TestWalkIntegration_UnreadableEntries(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced for the current user")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "x.txt"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unreadable.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "z.txt"), nil, 0o600))
	require.NoError(t, os.Chmod(locked, 0))
	require.NoError(t, os.Chmod(filepath.Join(root, "unreadable.txt"), 0))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	got := walkTree(t, root, files.WithRecursive())
	require.Equal(t, []string{"locked", "unreadable.txt", "z.txt"}, got.paths, "the walk continues past the locked directory")
	require.Len(t, got.errs, 1)
	require.ErrorIs(t, got.errs[""], fs.ErrPermission)

	_, err := files.ReadRegular(filepath.Join(root, "unreadable.txt"))
	require.ErrorIs(t, err, fs.ErrPermission)
}

// TestWalkIntegration_OpenInsideRoot opens files of the real tree through an
// os.Root: links that stay inside the root resolve, the link escaping it
// does not, and a directory is refused as not regular.
func TestWalkIntegration_OpenInsideRoot(t *testing.T) {
	t.Parallel()
	root := realTree(t)
	r, err := files.OpenRootDir(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	f, err := files.OpenRegularIn(r, "link.txt")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = files.OpenRegularIn(r, "outside/secret.txt")
	require.Error(t, err, "a link leading outside the root must not open")

	_, err = files.OpenRegularIn(r, "sub")
	require.ErrorIs(t, err, files.ErrNotRegular)

	data, err := files.ReadRegular(filepath.Join(root, "link.txt"))
	require.NoError(t, err)
	require.Equal(t, "root/sub/b.txt", string(data), "ReadRegular follows a link to a regular file")
}
