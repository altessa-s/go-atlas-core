//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that check the kernel enforces an applied Landlock
// ruleset on a real directory tree: read-write paths accept writes,
// read-only paths reject them, everything else rejects reads too, and
// a program exec'd from the restricted thread inherits the domain.
// Each test runs in a re-executed child because Apply is irreversible;
// on kernels without Landlock the child returns without asserting.
// The domain lands on the calling thread only, so every body holds
// runtime.LockOSThread from Apply through the last assertion: a
// migrated goroutine would probe the filesystem, or fork the exec'd
// program, from an unrestricted peer thread.

package landlock_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
	"github.com/altessa-s/go-atlas-core/runtime/landlock"
)

// landlockDirEnv carries the parent's t.TempDir into the child.
const landlockDirEnv = "GO_ATLAS_LANDLOCK_ITEST_DIR"

// landlockTree returns the rw, ro and out directories of a tree under
// a temporary directory owned by the parent test. The parent creates
// the tree and exports its root; the re-executed child inherits the
// variable and reuses it, so the parent's cleanup removes whatever the
// child wrote. Tests that call it cannot run in parallel because of
// t.Setenv.
func landlockTree(t *testing.T) (rw, ro, out string) {
	t.Helper()
	root := os.Getenv(landlockDirEnv)
	if root == "" {
		root = t.TempDir()
		t.Setenv(landlockDirEnv, root)
		for _, name := range []string{"rw", "ro", "out"} {
			dir := filepath.Join(root, name)
			if err := os.Mkdir(dir, 0o750); err != nil {
				t.Fatalf("Mkdir(%s): %v", dir, err)
			}
			if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(name), 0o600); err != nil {
				t.Fatalf("WriteFile in %s: %v", dir, err)
			}
		}
	}
	return filepath.Join(root, "rw"), filepath.Join(root, "ro"), filepath.Join(root, "out")
}

// systemReadPaths returns the system directories a dynamically linked
// program needs to load, keeping only those that exist on this host.
func systemReadPaths() []string {
	var paths []string
	for _, p := range []string{"/bin", "/usr", "/lib", "/lib64", "/etc", "/dev", "/proc"} {
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

func requireDenied(op string, err error) {
	if err == nil {
		exectest.Failf("%s succeeded outside the Landlock allowlist", op)
	}
	if !errors.Is(err, os.ErrPermission) {
		exectest.Failf("%s returned %v, want a permission error", op, err)
	}
}

func TestApply_EnforcesReadAndWriteRights(t *testing.T) {
	rw, ro, out := landlockTree(t)
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if !landlock.Supported() {
			return
		}
		if err := landlock.Apply(
			landlock.WithReadPaths(ro),
			landlock.WithReadPaths(systemReadPaths()...),
			landlock.WithReadWritePaths(rw),
		); err != nil {
			exectest.Failf("Apply: %v", err)
		}

		// Read-write path: create, write, read back, rename, remove.
		created := filepath.Join(rw, "new.txt")
		if err := os.WriteFile(created, []byte("data"), 0o600); err != nil {
			exectest.Failf("create in read-write path: %v", err)
		}
		if got, err := os.ReadFile(created); err != nil || string(got) != "data" {
			exectest.Failf("read back in read-write path = (%q, %v)", got, err)
		}
		if err := os.Mkdir(filepath.Join(rw, "sub"), 0o750); err != nil {
			exectest.Failf("mkdir in read-write path: %v", err)
		}
		if err := os.Remove(created); err != nil {
			exectest.Failf("remove in read-write path: %v", err)
		}

		// Read-only path: reads and listings work, every write is denied.
		if got, err := os.ReadFile(filepath.Join(ro, "file.txt")); err != nil || string(got) != "ro" {
			exectest.Failf("read in read-only path = (%q, %v)", got, err)
		}
		if _, err := os.ReadDir(ro); err != nil {
			exectest.Failf("list read-only path: %v", err)
		}
		requireDenied("write in read-only path",
			os.WriteFile(filepath.Join(ro, "file.txt"), []byte("x"), 0o600))
		requireDenied("create in read-only path",
			os.WriteFile(filepath.Join(ro, "new.txt"), []byte("x"), 0o600))
		requireDenied("mkdir in read-only path", os.Mkdir(filepath.Join(ro, "sub"), 0o750))
		requireDenied("remove in read-only path", os.Remove(filepath.Join(ro, "file.txt")))

		// Outside the allowlist: reads are denied too.
		_, err := os.ReadFile(filepath.Join(out, "file.txt"))
		requireDenied("read outside the allowlist", err)
		_, err = os.ReadDir(out)
		requireDenied("list outside the allowlist", err)
		requireDenied("create outside the allowlist",
			os.WriteFile(filepath.Join(out, "new.txt"), []byte("x"), 0o600))
	})
}

func TestApply_ExecInheritsDomain(t *testing.T) {
	rw, _, out := landlockTree(t)
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if !landlock.Supported() {
			return
		}
		cat, err := exec.LookPath("cat")
		if err != nil {
			// No cat in the image: nothing to exec into.
			return
		}
		if err := landlock.Apply(
			landlock.WithReadPaths(systemReadPaths()...),
			landlock.WithReadWritePaths(rw),
		); err != nil {
			exectest.Failf("Apply: %v", err)
		}

		// #nosec G204 -- fixed binary and paths created by the test
		got, err := exec.CommandContext(t.Context(), cat, filepath.Join(rw, "file.txt")).Output()
		if err != nil || string(got) != "rw" {
			exectest.Failf("exec'd cat inside the allowlist = (%q, %v)", got, err)
		}

		// #nosec G204 -- fixed binary and paths created by the test
		got, err = exec.CommandContext(t.Context(), cat, filepath.Join(out, "file.txt")).Output()
		if err == nil {
			exectest.Failf("exec'd cat read %q outside the allowlist: the domain was not inherited", got)
		}
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			exectest.Failf("exec'd cat outside the allowlist: %v, want a non-zero exit", err)
		}
	})
}
