// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build linux

// Subprocess tests for the irreversible Linux primitives in this
// package. capset(2) drops are irreversible for an unprivileged
// process and bounding-set drops are irreversible even for root, so a
// successful DropAll/DropAllExcept call would contaminate every later
// test in the same binary. These tests run in a re-execed child that
// exits immediately after the assertion.
//
// Capability sets are per thread, and the package releases its OS-thread
// pin when a call returns. Every body therefore locks its OS thread
// first and keeps it through setup, mutations and assertions, so each
// call and each snapshot sees the same thread.

package capabilities_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/capabilities"
	"github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
)

// canDropBounding reports whether s holds CAP_SETPCAP in its permitted
// set, which PR_CAPBSET_DROP needs in the effective set.
func canDropBounding(s capabilities.Sets) bool {
	return s.Has(capabilities.CAP_SETPCAP, capabilities.Permitted)
}

// getSets reads the calling thread's snapshot or fails the child.
func getSets(when string) capabilities.Sets {
	s, err := capabilities.Get()
	if err != nil {
		exectest.Failf("Get %s: %v", when, err)
	}
	return s
}

// requireRejectedUnchanged requires err to be ErrInvalidOption and the
// thread's capabilities to still equal before: a request the kernel
// would refuse must be rejected before any set is changed.
func requireRejectedUnchanged(op string, err error, before capabilities.Sets) {
	if !errors.Is(err, capabilities.ErrInvalidOption) {
		exectest.Failf("%s returned %v, want ErrInvalidOption", op, err)
	}
	if after := getSets("after the rejected " + op); after != before {
		exectest.Failf("%s was rejected but changed the thread: before=%+v after=%+v", op, before, after)
	}
}

// TestDropAll_ZeroesEverySetInSubprocess verifies that DropAll
// actually clears effective/permitted/inheritable/bounding/ambient on
// the calling thread, against a real kernel, through the whole Apply
// sequence.
//
// Dropping bounding bits needs CAP_SETPCAP. A runner that holds it
// (root, including a default Docker container) must end with every set
// zero; that is the case the bounding drop used to fail with EPERM,
// because capset had already removed CAP_SETPCAP from the effective
// set. A runner without it (an unprivileged user with a non-empty
// bounding set) must get ErrInvalidOption with its sets untouched. A
// runner whose bounding set is already empty needs no privilege at all.
func TestDropAll_ZeroesEverySetInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := getSets("before DropAll")
		err := capabilities.DropAll()
		if before.Bounding != 0 && !canDropBounding(before) {
			requireRejectedUnchanged("DropAll", err, before)
			return
		}
		if err != nil {
			exectest.Failf("DropAll: %v", err)
		}
		if got := getSets("after DropAll"); got != (capabilities.Sets{}) {
			exectest.Failf("sets after DropAll = %+v, want all zero "+
				"(bounding drop is irreversible — DropAll must clear it)", got)
		}
	})
}

// TestDropAllExcept_KeepsListedAndDropsRestInSubprocess covers the
// "bind a privileged port then drop everything else" pattern. It asks
// to keep CAP_NET_BIND_SERVICE and asserts that:
//
//   - the kept cap stays in the bounding set when it was there, and is
//     effective and permitted when it was permitted ("you cannot keep
//     what you do not have": a runner without it ends with every set
//     zero instead of an error);
//   - every other bit is zero.
//
// The CAP_SETPCAP rules of TestDropAll_ZeroesEverySetInSubprocess
// apply to the bits that leave the bounding set.
func TestDropAllExcept_KeepsListedAndDropsRestInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := getSets("before DropAllExcept")
		bit := uint64(1) << capabilities.CAP_NET_BIND_SERVICE

		err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE)
		if before.Bounding&^bit != 0 && !canDropBounding(before) {
			requireRejectedUnchanged("DropAllExcept", err, before)
			return
		}
		if err != nil {
			exectest.Failf("DropAllExcept(CAP_NET_BIND_SERVICE): %v", err)
		}
		want := capabilities.Sets{
			Effective: before.Permitted & bit,
			Permitted: before.Permitted & bit,
			Bounding:  before.Bounding & bit,
		}
		if got := getSets("after DropAllExcept"); got != want {
			exectest.Failf("sets after DropAllExcept = %+v, want %+v", got, want)
		}
	})
}

// TestApply_BoundingDropWithSetpcapOnlyPermittedInSubprocess covers a
// thread that holds CAP_SETPCAP in its permitted set but not in its
// effective set: Apply must still drop bounding bits, raising
// CAP_SETPCAP for the drop only, and end with exactly the requested
// sets, CAP_SETPCAP not effective. Skipped without CAP_SETPCAP.
func TestApply_BoundingDropWithSetpcapOnlyPermittedInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		cur := getSets("before Apply")
		chown := uint64(1) << capabilities.CAP_CHOWN
		if !canDropBounding(cur) || cur.Bounding&chown == 0 {
			return
		}
		setpcap := uint64(1) << capabilities.CAP_SETPCAP

		lowered := cur
		lowered.Effective &^= setpcap
		if err := capabilities.Apply(lowered); err != nil {
			exectest.Failf("Apply(CAP_SETPCAP not effective): %v", err)
		}
		mid := getSets("after lowering CAP_SETPCAP")
		if mid != lowered || mid.Has(capabilities.CAP_SETPCAP, capabilities.Effective) ||
			!mid.Has(capabilities.CAP_SETPCAP, capabilities.Permitted) {
			exectest.Failf("sets after lowering CAP_SETPCAP = %+v, want %+v "+
				"(CAP_SETPCAP permitted but not effective)", mid, lowered)
		}

		want := lowered
		want.Effective &^= chown
		want.Permitted &^= chown
		want.Inheritable &^= chown
		want.Ambient &^= chown
		want.Bounding &^= chown
		if err := capabilities.Apply(want); err != nil {
			exectest.Failf("Apply(drop CAP_CHOWN from bounding): %v", err)
		}
		if got := getSets("after Apply"); got != want {
			exectest.Failf("sets after Apply = %+v, want %+v", got, want)
		}
	})
}

// TestApply_EffectiveOutsidePermittedRejectedInSubprocess requests
// CAP_SETPCAP effective but not permitted, with every bounding bit
// dropped. capset refuses an effective bit outside the new permitted
// set, so Apply must reject the snapshot before touching anything —
// in particular before the irreversible bounding drops, which a thread
// holding CAP_SETPCAP could otherwise perform.
func TestApply_EffectiveOutsidePermittedRejectedInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := getSets("before Apply")
		err := capabilities.Apply(capabilities.Sets{Effective: uint64(1) << capabilities.CAP_SETPCAP})
		requireRejectedUnchanged("Apply(effective outside permitted)", err, before)
	})
}

// TestApply_InheritableRulesInSubprocess pins the capset rule for new
// inheritable bits: without CAP_SETPCAP effective a bit may be added
// only from the current inheritable or permitted set, and with it any
// bit of the bounding set may be added. It lowers CAP_CHOWN from the
// effective and permitted sets and CAP_SETPCAP from the effective set,
// requires adding CAP_CHOWN to inheritable to be rejected with the
// thread unchanged, then raises CAP_SETPCAP and requires the same
// request to succeed. Needs CAP_SETPCAP permitted and CAP_CHOWN
// permitted and in the bounding set; returns without asserting
// otherwise.
func TestApply_InheritableRulesInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		cur := getSets("before Apply")
		if !cur.Has(capabilities.CAP_SETPCAP, capabilities.Permitted) ||
			!cur.Has(capabilities.CAP_CHOWN, capabilities.Permitted) ||
			!cur.Has(capabilities.CAP_CHOWN, capabilities.Bounding) ||
			cur.Has(capabilities.CAP_CHOWN, capabilities.Inheritable) {
			return
		}
		chown := uint64(1) << capabilities.CAP_CHOWN
		setpcap := uint64(1) << capabilities.CAP_SETPCAP

		lowered := cur
		lowered.Effective &^= chown | setpcap
		lowered.Permitted &^= chown
		lowered.Ambient &^= chown
		if err := capabilities.Apply(lowered); err != nil {
			exectest.Failf("Apply(lower CAP_CHOWN and CAP_SETPCAP): %v", err)
		}
		if got := getSets("after lowering"); got != lowered {
			exectest.Failf("sets after lowering = %+v, want %+v", got, lowered)
		}

		capped := lowered
		capped.Inheritable |= chown
		err := capabilities.Apply(capped)
		requireRejectedUnchanged("Apply(inheritable outside permitted without CAP_SETPCAP)", err, lowered)

		// The kernel checks CAP_SETPCAP in the effective set the thread
		// has before the call, so it has to be raised by a call of its
		// own first.
		raised := lowered
		raised.Effective |= setpcap
		if err := capabilities.Apply(raised); err != nil {
			exectest.Failf("Apply(raise CAP_SETPCAP): %v", err)
		}
		raised.Inheritable |= chown
		if err := capabilities.Apply(raised); err != nil {
			exectest.Failf("Apply(inheritable from bounding with CAP_SETPCAP): %v", err)
		}
		if got := getSets("after raising"); got != raised {
			exectest.Failf("sets after raising = %+v, want %+v", got, raised)
		}
	})
}

// TestDropAllExcept_KeepsPermittedOutsideBoundingInSubprocess drops
// CAP_NET_BIND_SERVICE from the bounding set while it stays permitted,
// which Linux allows, and then keeps it with DropAllExcept: the result
// must hold it effective and permitted with an empty bounding set.
// Needs CAP_SETPCAP effective and CAP_NET_BIND_SERVICE permitted and in
// the bounding set; returns without asserting otherwise.
func TestDropAllExcept_KeepsPermittedOutsideBoundingInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		before := getSets("before BoundingDrop")
		if !before.Has(capabilities.CAP_SETPCAP, capabilities.Effective) ||
			!before.Has(capabilities.CAP_NET_BIND_SERVICE, capabilities.Permitted) ||
			!before.Has(capabilities.CAP_NET_BIND_SERVICE, capabilities.Bounding) {
			return
		}
		bit := uint64(1) << capabilities.CAP_NET_BIND_SERVICE

		if err := capabilities.BoundingDrop(capabilities.CAP_NET_BIND_SERVICE); err != nil {
			exectest.Failf("BoundingDrop(CAP_NET_BIND_SERVICE): %v", err)
		}
		mid := before
		mid.Bounding &^= bit
		if got := getSets("after BoundingDrop"); got != mid {
			exectest.Failf("sets after BoundingDrop = %+v, want %+v", got, mid)
		}

		if err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE); err != nil {
			exectest.Failf("DropAllExcept(CAP_NET_BIND_SERVICE) after BoundingDrop: %v", err)
		}
		want := capabilities.Sets{Effective: bit, Permitted: bit}
		if got := getSets("after DropAllExcept"); got != want {
			exectest.Failf("sets after DropAllExcept = %+v, want %+v", got, want)
		}
	})
}

// TestGet_ReturnsKernelStateInSubprocess asserts that Get() reads
// the kernel-side capability state via capget(2) without mutating
// it. This is the read-side smoke test the rest of the suite relies
// on for its post-condition assertions — if Get is broken, every
// other subprocess test in this file is unsound.
func TestGet_ReturnsKernelStateInSubprocess(t *testing.T) {
	exectest.RunInSubprocess(t, func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		first, err := capabilities.Get()
		if err != nil {
			exectest.Failf("first Get: %v", err)
		}
		second, err := capabilities.Get()
		if err != nil {
			exectest.Failf("second Get: %v", err)
		}
		// Two consecutive Get calls without intervening mutation
		// must return identical snapshots. If they differ, capget
		// is reading uninitialized memory or the snapshot type is
		// not stable.
		if first != second {
			exectest.Failf("Get returned non-stable snapshot: first=%+v second=%+v",
				first, second)
		}
	})
}
