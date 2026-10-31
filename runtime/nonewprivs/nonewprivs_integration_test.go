//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that check the no_new_privs bit Set installs is
// visible to the kernel's /proc accounting and survives execve(2) into
// a child program. Each test runs in a re-executed child because the
// bit is irreversible.

package nonewprivs_test

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
	"github.com/altessa-s/go-atlas-core/runtime/nonewprivs"
)

// noNewPrivsField returns the NoNewPrivs value of a /proc status file.
func noNewPrivsField(status []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(status))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "NoNewPrivs:"); ok {
			return strings.TrimSpace(v)
		}
	}
	exectest.Failf("NoNewPrivs not found in status:\n%s", status)
	return ""
}

func TestSet_VisibleInThreadStatus(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := nonewprivs.Set(); err != nil {
			exectest.Failf("Set: %v", err)
		}
		status, err := os.ReadFile("/proc/thread-self/status")
		if err != nil {
			exectest.Failf("read /proc/thread-self/status: %v", err)
		}
		if got := noNewPrivsField(status); got != "1" {
			exectest.Failf("NoNewPrivs of the calling thread = %q after Set, want 1", got)
		}
	})
}

func TestSet_InheritedByExecdChild(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		cat, err := exec.LookPath("cat")
		if err != nil {
			// No cat in the image: nothing to exec into.
			return
		}
		// Fork happens on the calling thread; pin it so the child is
		// cloned from the thread that has the bit.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := nonewprivs.Set(); err != nil {
			exectest.Failf("Set: %v", err)
		}
		// cat reads its own status, so the field reports the exec'd
		// program's bit.
		// #nosec G204 -- fixed binary and argument
		status, err := exec.CommandContext(t.Context(), cat, "/proc/self/status").Output()
		if err != nil {
			exectest.Failf("exec %s: %v", cat, err)
		}
		if got := noNewPrivsField(status); got != "1" {
			exectest.Failf("NoNewPrivs of the exec'd child = %q, want 1", got)
		}
	})
}
