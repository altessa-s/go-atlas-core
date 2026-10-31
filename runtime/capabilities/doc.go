// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package capabilities wraps the Linux capabilities subsystem
// (capset(2), capget(2), prctl(2) PR_CAPBSET_*, PR_CAP_AMBIENT_*). It
// lets a process drop capabilities at startup and confine itself to a
// strict allowlist without requiring cgo or libcap.
//
// # Quick start
//
// Every call changes the calling OS thread only, so lock the thread
// from the drop through the restricted work and keep that work on this
// goroutine (goroutines it starts run on other threads, which keep
// their capabilities). To restrict a whole service, drop capabilities
// when it is launched; see Caveats.
//
// Drop every capability of this thread before running untrusted code
// on it:
//
//	runtime.LockOSThread()
//	defer runtime.UnlockOSThread()
//	if err := capabilities.DropAll(); err != nil {
//	    log.Fatalf("capabilities: %v", err)
//	}
//	runRestricted()
//
// Preserve one capability on this thread (the "bind a privileged port
// then drop everything else" pattern):
//
//	runtime.LockOSThread()
//	defer runtime.UnlockOSThread()
//	if err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE); err != nil {
//	    log.Fatalf("capabilities: %v", err)
//	}
//
// # Four-set model
//
// Linux capabilities are not a single mask but five related sets
// tracked per thread:
//
//   - Effective   — the bits currently in force for privileged syscalls.
//   - Permitted   — the ceiling from which effective can be raised.
//   - Inheritable — passed to children across execve; rarely needed.
//   - Bounding    — the ceiling from which permitted can be raised; a
//     bit dropped from bounding can never be re-acquired.
//   - Ambient     — preserved across a non-privileged execve when also
//     present in permitted and inheritable.
//
// [Sets] reports all five as 64-bit masks. [Apply] accepts a full
// snapshot and installs it in a validated, ordered syscall sequence;
// [DropAll] / [DropAllExcept] are thin wrappers implementing the two
// most common startup patterns.
//
// # Threat model
//
// This package is a defense-in-depth primitive. It restricts the
// capability set visible to subsequent syscalls. It does NOT:
//
//   - Sandbox filesystem access (use
//     [github.com/altessa-s/go-atlas-core/runtime/landlock]).
//   - Cap resource usage (use
//     [github.com/altessa-s/go-atlas-core/runtime/rlimits]).
//   - Defeat SUID escalation on exec (use
//     [github.com/altessa-s/go-atlas-core/runtime/nonewprivs]).
//   - Protect against a malicious binary already loaded in-process —
//     such a binary still has full access to host memory.
//
// Combine this package with the others for a layered startup hardening
// sequence: nonewprivs → rlimits → capabilities → seccomp → landlock.
//
// # Caveats
//
//   - Linux only. On non-Linux platforms every function returns
//     [ErrUnsupported] or its zero-value equivalent so cross-platform
//     code can import the package without build tags.
//   - Per-thread scope. capset(2) mutates only the calling thread. The
//     package pins the goroutine to its OS thread for the duration of
//     [Apply], [DropAll] and [DropAllExcept], but the change lands on
//     that thread only: even at the start of main the Go runtime already
//     runs peer OS threads that keep their capabilities, so calling
//     before spawning goroutines does not make the drop process-wide.
//     For a process-wide drop, drop capabilities externally before the
//     Go binary starts (systemd CapabilityBoundingSet=, container
//     --cap-drop, or a launcher that drops them before execve(2)); work
//     restricted by this package must stay pinned to the thread that
//     made the call. The pin ends when the call returns, and [Get],
//     [BoundingDrop] and the ambient helpers do not pin at all, so a
//     sequence of calls that builds on earlier results (Get then Apply,
//     BoundingDrop then DropAllExcept) must run under
//     [runtime.LockOSThread] to stay on one thread.
//   - Irreversible ceilings. Bits dropped from the bounding set cannot
//     be raised by any descendant, and the inheritable/ambient
//     invariants are enforced by the kernel — not by this package.
//   - Raising bits is possible only within the current ceilings.
//     Dropping effective/permitted/inheritable/ambient bits is always
//     possible for an unprivileged process; dropping bounding bits
//     needs CAP_SETPCAP in the permitted set. [Apply], [DropAll] and
//     [DropAllExcept] check the capset(2) rules and that requirement
//     up front and reject a request that breaks them with
//     [ErrInvalidOption] before changing anything. Permitted bits
//     outside the bounding set stay allowed.
package capabilities
