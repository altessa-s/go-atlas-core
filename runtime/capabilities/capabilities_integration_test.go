//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that check dropped capabilities end to end: the
// kernel's /proc accounting agrees with Get, an operation guarded by a
// dropped capability fails, and a program exec'd afterwards cannot get
// the capability back even as root. Each test runs in a re-executed
// child because drops are irreversible; a child that does not start
// with CAP_CHOWN (an unprivileged runner) returns without asserting.

package capabilities_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/capabilities"
	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
)

// capabilitiesDirEnv carries the parent's t.TempDir into the child.
const capabilitiesDirEnv = "GO_ATLAS_CAPABILITIES_ITEST_DIR"

// nobody is the uid and gid the chown probes hand files to.
const nobody = 65534

// sharedTempDir returns a temporary directory owned by the parent test.
// The parent creates it with t.TempDir and exports it; the re-executed
// child inherits the variable and reuses the directory, so the parent's
// cleanup removes whatever the child wrote. Tests that call it cannot
// run in parallel because of t.Setenv.
func sharedTempDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv(capabilitiesDirEnv); dir != "" {
		return dir
	}
	dir := t.TempDir()
	t.Setenv(capabilitiesDirEnv, dir)
	return dir
}

// statusMasks returns the Cap* hex masks of a /proc status file.
func statusMasks(status []byte) map[string]uint64 {
	masks := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(status))
	for sc.Scan() {
		name, value, ok := strings.Cut(sc.Text(), ":")
		if !ok || !strings.HasPrefix(name, "Cap") {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		if err != nil {
			exectest.Failf("parse %s %q: %v", name, value, err)
		}
		masks[name] = v
	}
	return masks
}

// hasChown reports whether the calling thread starts with CAP_CHOWN in
// its effective set.
func hasChown() bool {
	s, err := capabilities.Get()
	if err != nil {
		exectest.Failf("Get: %v", err)
	}
	return s.Has(capabilities.CAP_CHOWN, capabilities.Effective)
}

// newFile creates a file in dir for a chown probe.
func newFile(dir, name string) string {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		exectest.Failf("WriteFile(%s): %v", path, err)
	}
	return path
}

// threadMasks returns the Cap* masks of the calling thread from /proc.
func threadMasks() map[string]uint64 {
	status, err := os.ReadFile("/proc/thread-self/status")
	if err != nil {
		exectest.Failf("read /proc/thread-self/status: %v", err)
	}
	return statusMasks(status)
}

// requireMatchesProc requires the /proc accounting of the calling
// thread to agree with s.
func requireMatchesProc(s capabilities.Sets) {
	masks := threadMasks()
	want := map[string]uint64{
		"CapEff": s.Effective,
		"CapPrm": s.Permitted,
		"CapInh": s.Inheritable,
		"CapBnd": s.Bounding,
		"CapAmb": s.Ambient,
	}
	for name, w := range want {
		if masks[name] != w {
			exectest.Failf("%s = %#x in /proc, want %#x", name, masks[name], w)
		}
	}
}

func TestApply_EffectiveDropDeniesChownUntilRaised(t *testing.T) {
	dir := sharedTempDir(t)
	exectest.RunInSubprocess(t, func() {
		if !hasChown() {
			return
		}
		path := newFile(dir, "probe")
		cur, err := capabilities.Get()
		if err != nil {
			exectest.Failf("Get: %v", err)
		}

		// Lower CAP_CHOWN from the effective set only: the bounding set
		// is unchanged, so no bounding drop is involved, and the bit
		// stays in the permitted set so it can be raised again.
		chown := uint64(1) << capabilities.CAP_CHOWN
		lowered := cur
		lowered.Effective &^= chown
		if err := capabilities.Apply(lowered); err != nil {
			exectest.Failf("Apply(lowered): %v", err)
		}
		requireMatchesProc(lowered)
		err = os.Chown(path, nobody, nobody)
		if !errors.Is(err, syscall.EPERM) {
			exectest.Failf("chown without CAP_CHOWN in effective returned %v, want EPERM", err)
		}

		if err := capabilities.Apply(cur); err != nil {
			exectest.Failf("Apply(raised): %v", err)
		}
		requireMatchesProc(cur)
		if err := os.Chown(path, nobody, nobody); err != nil {
			exectest.Failf("chown after raising CAP_CHOWN again: %v", err)
		}
	})
}

func TestApply_PermittedDropIsPermanent(t *testing.T) {
	dir := sharedTempDir(t)
	exectest.RunInSubprocess(t, func() {
		if !hasChown() {
			return
		}
		path := newFile(dir, "permanent")
		cur, err := capabilities.Get()
		if err != nil {
			exectest.Failf("Get: %v", err)
		}

		chown := uint64(1) << capabilities.CAP_CHOWN
		dropped := cur
		dropped.Effective &^= chown
		dropped.Permitted &^= chown
		dropped.Inheritable &^= chown
		dropped.Ambient &^= chown
		if err := capabilities.Apply(dropped); err != nil {
			exectest.Failf("Apply(dropped): %v", err)
		}
		requireMatchesProc(dropped)

		// The kernel refuses to raise a bit that left the permitted set.
		err = capabilities.Apply(cur)
		if !errors.Is(err, capabilities.ErrFailed) || !errors.Is(err, syscall.EPERM) {
			exectest.Failf("re-raising CAP_CHOWN after dropping it from permitted returned %v, want ErrFailed wrapping EPERM", err)
		}
		if err := os.Chown(path, nobody, nobody); !errors.Is(err, syscall.EPERM) {
			exectest.Failf("chown after dropping CAP_CHOWN from permitted returned %v, want EPERM", err)
		}
	})
}

func TestBoundingDrop_ExecdChildCannotRegainCapability(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		if !hasChown() {
			return
		}
		cat, err := exec.LookPath("cat")
		if err != nil {
			// No cat in the image: nothing to exec into.
			return
		}
		// The bounding set is per thread and BoundingDrop does not pin
		// the goroutine: hold the thread so the fork below clones the
		// thread that dropped the capability.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := capabilities.BoundingDrop(capabilities.CAP_CHOWN); err != nil {
			exectest.Failf("BoundingDrop(CAP_CHOWN): %v", err)
		}

		// Without the bounding drop, a root child would get CAP_CHOWN
		// back in its permitted and effective sets on execve.
		// #nosec G204 -- fixed binary and argument
		status, err := exec.CommandContext(t.Context(), cat, "/proc/self/status").Output()
		if err != nil {
			exectest.Failf("exec %s: %v", cat, err)
		}
		masks := statusMasks(status)
		chown := uint64(1) << capabilities.CAP_CHOWN
		for _, name := range []string{"CapBnd", "CapPrm", "CapEff"} {
			if masks[name]&chown != 0 {
				exectest.Failf("exec'd child has CAP_CHOWN in %s = %#x after BoundingDrop", name, masks[name])
			}
		}
	})
}
