// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package exectest provides a helper for tests that need to execute
// syscalls with irreversible process-wide side effects — Linux security
// primitives like capset(2), prctl(PR_SET_NO_NEW_PRIVS),
// landlock_restrict_self(2), seccomp(SET_MODE_FILTER), and persistent
// setrlimit calls. The kernel does not let a process undo any of those
// once applied, so running them in the test binary itself would
// contaminate every later test.
//
// The helper re-execs the test binary in a subprocess so the side
// effect is contained to a child that exits immediately afterward.
// Use it like this:
//
//	func TestDropAllSubprocess(t *testing.T) {
//	    exectest.RunInSubprocess(t, func() {
//	        if err := capabilities.DropAll(); err != nil {
//	            exectest.Failf("DropAll: %v", err)
//	        }
//	        snap, err := capabilities.Get()
//	        if err != nil {
//	            exectest.Failf("Get after DropAll: %v", err)
//	        }
//	        if snap.Effective != 0 {
//	            exectest.Failf("Effective != 0 after DropAll: %#x", snap.Effective)
//	        }
//	    })
//	}
//
// The same test function plays both roles. The parent invocation
// (under `go test`) re-execs the binary with -test.run=^TestName$ and
// the GO_ATLAS_EXECTEST_CHILD env var set; the child invocation
// detects the env var and runs fn directly. Standard testing.T methods
// do NOT cross the process boundary — use [Failf] inside fn to report
// failures.
//
// RunInSubprocess only runs on Linux; on every other platform it calls
// t.Skip. Tests that depend on this helper should live in
// `_linux_test.go` files so the build never compiles them on macOS or
// Windows.
package exectest
