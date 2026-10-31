//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that check the kernel enforces the limits Apply
// installs: the operation the limit guards fails with the documented
// errno, and the limit survives execve(2) into a child process. Each
// test runs in a re-executed child because a lowered limit cannot be
// raised again.

package rlimits_test

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
	"github.com/altessa-s/go-atlas-core/runtime/rlimits"
)

// rlimitsDirEnv carries the parent's t.TempDir into the child.
const rlimitsDirEnv = "GO_ATLAS_RLIMITS_ITEST_DIR"

// sharedTempDir returns a temporary directory owned by the parent test.
// The parent creates it with t.TempDir and exports it; the re-executed
// child inherits the variable and reuses the directory, so the parent's
// cleanup removes whatever the child wrote. Tests that call it cannot
// run in parallel because of t.Setenv.
func sharedTempDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv(rlimitsDirEnv); dir != "" {
		return dir
	}
	dir := t.TempDir()
	t.Setenv(rlimitsDirEnv, dir)
	return dir
}

func TestApply_OpenFilesLimitRejectsExtraDescriptors(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		const limit = 64
		if err := rlimits.Apply(rlimits.WithMaxOpenFiles(limit)); err != nil {
			exectest.Failf("Apply(WithMaxOpenFiles(%d)): %v", limit, err)
		}

		var opened []*os.File
		var openErr error
		for range limit + 1 {
			f, err := os.Open(os.DevNull)
			if err != nil {
				openErr = err
				break
			}
			opened = append(opened, f)
		}
		if openErr == nil {
			exectest.Failf("opened %d descriptors under RLIMIT_NOFILE=%d", len(opened), limit)
		}
		if !errors.Is(openErr, syscall.EMFILE) {
			exectest.Failf("open past the limit returned %v, want EMFILE", openErr)
		}
		if len(opened) == 0 {
			exectest.Failf("no descriptor could be opened under RLIMIT_NOFILE=%d", limit)
		}

		// Closing a descriptor frees a slot under the same limit.
		if err := opened[len(opened)-1].Close(); err != nil {
			exectest.Failf("Close: %v", err)
		}
		f, err := os.Open(os.DevNull)
		if err != nil {
			exectest.Failf("open after freeing a slot: %v", err)
		}
		_ = f.Close()
	})
}

func TestApply_FileSizeLimitRejectsLargeWrites(t *testing.T) {
	dir := sharedTempDir(t)
	exectest.RunInSubprocess(t, func() {
		const limit = 4096
		if err := rlimits.Apply(rlimits.WithMaxFileSizeBytes(limit)); err != nil {
			exectest.Failf("Apply(WithMaxFileSizeBytes(%d)): %v", limit, err)
		}

		path := filepath.Join(dir, "big.bin")
		f, err := os.Create(path)
		if err != nil {
			exectest.Failf("Create: %v", err)
		}
		defer func() { _ = f.Close() }()

		// The Go runtime ignores SIGXFSZ, so the write past the limit
		// reports EFBIG instead of killing the process.
		n, err := f.Write(make([]byte, 2*limit))
		if !errors.Is(err, syscall.EFBIG) {
			exectest.Failf("write past RLIMIT_FSIZE returned (%d, %v), want EFBIG", n, err)
		}
		if n > limit {
			exectest.Failf("wrote %d bytes, more than RLIMIT_FSIZE=%d", n, limit)
		}
		info, err := f.Stat()
		if err != nil {
			exectest.Failf("Stat: %v", err)
		}
		if info.Size() > limit {
			exectest.Failf("file grew to %d bytes, more than RLIMIT_FSIZE=%d", info.Size(), limit)
		}
	})
}

func TestApply_AddressSpaceLimitRejectsLargeMappings(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		// The limit is relative to the current size because the Go
		// runtime (and the race detector) reserve large address ranges
		// up front; the headroom keeps the runtime working.
		const headroom = 256 << 20
		vmSize := statusKiB("VmSize") << 10
		limit := vmSize + headroom
		if err := rlimits.Apply(rlimits.WithMemoryBytes(limit)); err != nil {
			exectest.Failf("Apply(WithMemoryBytes(%d)): %v", limit, err)
		}

		mem, err := syscall.Mmap(-1, 0, 4*headroom, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
		if err == nil {
			_ = syscall.Munmap(mem)
			exectest.Failf("mapped %d bytes past RLIMIT_AS=%d", 4*headroom, limit)
		}
		if !errors.Is(err, syscall.ENOMEM) {
			exectest.Failf("mmap past RLIMIT_AS returned %v, want ENOMEM", err)
		}

		small, err := syscall.Mmap(-1, 0, 1<<20, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
		if err != nil {
			exectest.Failf("mmap within RLIMIT_AS: %v", err)
		}
		_ = syscall.Munmap(small)
	})
}

func TestApply_LimitsSurviveExec(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		sh, err := exec.LookPath("sh")
		if err != nil {
			// No shell in the image: nothing to exec into.
			return
		}
		const nofile = 128
		if err := rlimits.Apply(
			rlimits.WithMaxOpenFiles(nofile),
			rlimits.WithDisableCoreDumps(),
		); err != nil {
			exectest.Failf("Apply: %v", err)
		}

		// The Go runtime raises the soft RLIMIT_NOFILE at startup and
		// restores the original for exec'd children; a limit set
		// through Apply must reach the child instead.
		// #nosec G204 -- fixed shell script, no external input
		out, err := exec.CommandContext(t.Context(), sh, "-c", "ulimit -n; ulimit -c").Output()
		if err != nil {
			exectest.Failf("exec %s: %v", sh, err)
		}
		got := strings.Fields(string(out))
		want := []string{strconv.Itoa(nofile), "0"}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			exectest.Failf("limits in the exec'd child = %v, want %v", got, want)
		}
	})
}

// statusKiB returns a "kB" field of /proc/self/status as an int64.
func statusKiB(field string) int64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		exectest.Failf("open /proc/self/status: %v", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		value, ok := strings.CutPrefix(sc.Text(), field+":")
		if !ok {
			continue
		}
		kib, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(value), " kB"), 10, 64)
		if err != nil {
			exectest.Failf("parse %s %q: %v", field, value, err)
		}
		return kib
	}
	exectest.Failf("field %s not found in /proc/self/status", field)
	return 0
}
