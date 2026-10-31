//go:build integration && linux

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that check the installed filter end to end: every
// thread of the process, including threads created after the install,
// gets EPERM from denied syscalls, and a program exec'd after the
// install inherits the filter. Each test runs in a re-executed child
// because the filter is irreversible.

package seccomp_test

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
	"github.com/altessa-s/go-atlas-core/runtime/seccomp"

	"golang.org/x/sys/unix"
)

// seccompProbeEnv makes the re-executed test binary run one probe and
// exit, acting as the program exec'd after the filter is installed.
const seccompProbeEnv = "GO_ATLAS_SECCOMP_ITEST_PROBE"

// deniedProbe issues one syscall from the denylist with arguments that
// either succeed or fail with an errno other than EPERM when no filter
// is installed, so EPERM proves the filter answered.
type deniedProbe struct {
	name string
	call func() unix.Errno
}

func deniedProbes() []deniedProbe {
	raw := func(trap uintptr, args ...uintptr) func() unix.Errno {
		return func() unix.Errno {
			var a [6]uintptr
			copy(a[:], args)
			_, _, errno := unix.Syscall6(trap, a[0], a[1], a[2], a[3], a[4], a[5])
			return errno
		}
	}
	return []deniedProbe{
		// unshare(0) is a successful no-op without the filter.
		{name: "unshare", call: raw(unix.SYS_UNSHARE, 0)},
		// setns(-1, 0) fails with EBADF without the filter.
		{name: "setns", call: raw(unix.SYS_SETNS, ^uintptr(0), 0)},
		// bpf with an invalid command fails with EINVAL without the filter.
		{name: "bpf", call: raw(unix.SYS_BPF, ^uintptr(0), 0, 0)},
		// perf_event_open(NULL, ...) fails with EFAULT without the filter.
		{name: "perf_event_open", call: raw(unix.SYS_PERF_EVENT_OPEN, 0, 0, ^uintptr(0), ^uintptr(0), 0)},
		// keyctl with an invalid operation fails with EOPNOTSUPP without the filter.
		{name: "keyctl", call: raw(unix.SYS_KEYCTL, ^uintptr(0))},
		// userfaultfd with invalid flags fails with EINVAL without the filter.
		{name: "userfaultfd", call: raw(unix.SYS_USERFAULTFD, ^uintptr(0))},
	}
}

func TestBlockDangerousSyscalls_DeniesOnEveryThread(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		// Spread goroutines over OS threads that exist before the
		// install so TSYNC has peers to propagate to.
		const threads = 8
		probe := make(chan struct{})
		results := make(chan string, threads*len(deniedProbes()))
		var wg sync.WaitGroup
		var started sync.WaitGroup
		started.Add(threads)
		for range threads {
			wg.Go(func() {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				started.Done()
				<-probe
				for _, p := range deniedProbes() {
					if errno := p.call(); errno != unix.EPERM {
						results <- p.name + " on a peer thread: errno " + errno.Error()
					}
				}
			})
		}
		started.Wait()

		if err := seccomp.BlockDangerousSyscalls(); err != nil {
			exectest.Failf("BlockDangerousSyscalls: %v", err)
		}
		close(probe)
		wg.Wait()
		close(results)
		for r := range results {
			exectest.Failf("want EPERM: %s", r)
		}

		// A thread created after the install inherits the filter.
		done := make(chan unix.Errno)
		go func() {
			runtime.LockOSThread()
			// Never unlocked: the thread exits with the goroutine,
			// so the probe ran on a thread created for it.
			done <- deniedProbes()[0].call()
		}()
		if errno := <-done; errno != unix.EPERM {
			exectest.Failf("unshare on a new thread returned errno %v, want EPERM", errno)
		}
	})
}

func TestBlockDangerousSyscalls_ExecInheritsFilter(t *testing.T) {
	if os.Getenv(seccompProbeEnv) == "1" {
		// Exec'd probe: report through the exit status.
		for _, p := range deniedProbes() {
			if errno := p.call(); errno != unix.EPERM {
				os.Exit(3)
			}
		}
		os.Exit(0)
	}
	exectest.RunInSubprocess(t, func() {
		if err := seccomp.BlockDangerousSyscalls(); err != nil {
			exectest.Failf("BlockDangerousSyscalls: %v", err)
		}
		// #nosec G204 -- re-executes the test binary (os.Args[0]) as the probe
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), seccompProbeEnv+"=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			exectest.Failf("exec'd probe did not see EPERM from every denied syscall: %v\n%s", err, out)
		}
	})
}
