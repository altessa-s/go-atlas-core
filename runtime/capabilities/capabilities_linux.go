// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build linux

package capabilities

import (
	"errors"
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// capHeader returns a [unix.CapUserHeader] requesting version 3 (64-bit
// masks, kernel 2.6.26+). The Pid field is 0, meaning "the calling
// thread", which is the only safe target for unprivileged processes.
func capHeader() *unix.CapUserHeader {
	return &unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0,
	}
}

const (
	// capDataLen is the length of the [unix.CapUserData] array required
	// by capability version 3: two 32-bit words per set (low + high).
	capDataLen = 2

	// halfWord is the bit width of one capability data word. The kernel
	// stores each mask as two uint32 halves; shifting by halfWord moves
	// between the low and high words.
	halfWord = 32
)

// has reports whether c is set in mask.
func has(mask uint64, c Cap) bool {
	return mask&(uint64(1)<<uint(c)) != 0
}

// setBit returns mask with c added.
func setBit(mask uint64, c Cap) uint64 {
	return mask | (uint64(1) << uint(c))
}

// toData packs effective/permitted/inheritable masks into the kernel's
// two-word representation.
func toData(eff, perm, inh uint64) [capDataLen]unix.CapUserData {
	var d [capDataLen]unix.CapUserData
	d[0].Effective = uint32(eff)
	d[1].Effective = uint32(eff >> halfWord)
	d[0].Permitted = uint32(perm)
	d[1].Permitted = uint32(perm >> halfWord)
	d[0].Inheritable = uint32(inh)
	d[1].Inheritable = uint32(inh >> halfWord)
	return d
}

// fromData unpacks the kernel's two-word representation into uint64
// masks.
func fromData(d [capDataLen]unix.CapUserData) (eff, perm, inh uint64) {
	eff = uint64(d[0].Effective) | uint64(d[1].Effective)<<halfWord
	perm = uint64(d[0].Permitted) | uint64(d[1].Permitted)<<halfWord
	inh = uint64(d[0].Inheritable) | uint64(d[1].Inheritable)<<halfWord
	return eff, perm, inh
}

// capget calls capget(2) for the calling thread and returns the three
// thread-local masks. The bounding and ambient sets are read
// separately via prctl — see [readBounding] and [readAmbient].
func capget() (eff, perm, inh uint64, err error) {
	hdr := capHeader()
	var data [capDataLen]unix.CapUserData
	if e := unix.Capget(hdr, &data[0]); e != nil {
		return 0, 0, 0, fmt.Errorf("%w: capget: %w", ErrFailed, e)
	}
	eff, perm, inh = fromData(data)
	return eff, perm, inh, nil
}

// capset calls capset(2) for the calling thread, installing the given
// three thread-local masks.
func capset(eff, perm, inh uint64) error {
	hdr := capHeader()
	data := toData(eff, perm, inh)
	if err := unix.Capset(hdr, &data[0]); err != nil {
		return fmt.Errorf("%w: capset: %w", ErrFailed, err)
	}
	return nil
}

// readBounding probes the bounding set one cap at a time via
// PR_CAPBSET_READ. This is the canonical way to read the bounding set
// — there is no syscall that returns it as a mask.
func readBounding() (uint64, error) {
	var mask uint64
	for c := Cap(0); c <= capLastCap; c++ {
		v, err := unix.PrctlRetInt(unix.PR_CAPBSET_READ, uintptr(c), 0, 0, 0)
		if err != nil {
			return 0, fmt.Errorf("%w: prctl(PR_CAPBSET_READ, %s): %w", ErrFailed, c, err)
		}
		if v == 1 {
			mask = setBit(mask, c)
		}
	}
	return mask, nil
}

// readAmbient probes the ambient set one cap at a time via
// PR_CAP_AMBIENT_IS_SET. Same rationale as readBounding.
func readAmbient() (uint64, error) {
	var mask uint64
	for c := Cap(0); c <= capLastCap; c++ {
		v, err := unix.PrctlRetInt(
			unix.PR_CAP_AMBIENT,
			unix.PR_CAP_AMBIENT_IS_SET,
			uintptr(c), 0, 0,
		)
		if err != nil {
			return 0, fmt.Errorf("%w: prctl(PR_CAP_AMBIENT_IS_SET, %s): %w", ErrFailed, c, err)
		}
		if v == 1 {
			mask = setBit(mask, c)
		}
	}
	return mask, nil
}

// getSnapshot is the Linux implementation of [Get]. It reads all five
// sets on the calling thread. Pinning to the OS thread is not required
// for a read-only call — capget targets the thread but does not
// mutate any global state.
func getSnapshot() (Sets, error) {
	eff, perm, inh, err := capget()
	if err != nil {
		return Sets{}, err
	}
	bnd, err := readBounding()
	if err != nil {
		return Sets{}, err
	}
	amb, err := readAmbient()
	if err != nil {
		return Sets{}, err
	}
	return Sets{
		Effective:   eff,
		Permitted:   perm,
		Inheritable: inh,
		Bounding:    bnd,
		Ambient:     amb,
	}, nil
}

// applySnapshot is the Linux implementation of [Apply]: it installs s
// exactly. See [applyTarget] for the sequence and its guarantees.
func applySnapshot(s Sets) error {
	return applyTarget(func(Sets) Sets { return s })
}

// applyTarget installs the snapshot target(cur) on the calling thread,
// where cur is the thread's snapshot read under the same OS-thread pin.
// It:
//
//  1. Pins the goroutine to its OS thread for the duration.
//  2. Validates the target before changing anything (see
//     [validateTarget]): the capset(2) rules for the target
//     effective/permitted/inheritable masks, the target ambient mask
//     within its permitted and inheritable masks, and CAP_SETPCAP in
//     the current permitted set when bounding bits are dropped, since
//     PR_CAPBSET_DROP needs it in the effective set. A rejection
//     returns [ErrInvalidOption] and leaves the thread untouched.
//  3. Lowers the ambient bits the target drops. Lowering needs no
//     privilege.
//  4. Calls capset(2) with the target effective/permitted/inheritable,
//     keeping CAP_SETPCAP effective and permitted when bounding bits
//     still have to be dropped, even if the target removes it. Step 2
//     has already rejected every target the capset rules refuse, so a
//     kernel refusal here, before anything irreversible has happened,
//     comes from outside those rules (a security module, for one).
//  5. Drops every bounding bit not in the target. This is the only
//     irreversible step.
//  6. If step 4 kept CAP_SETPCAP beyond the target, calls capset(2)
//     again with exactly the target masks. This only removes bits.
//  7. Raises the target ambient bits that are not set yet.
//
// Guarantees on failure: a step-2 rejection changes nothing. A
// step-3 or step-4 failure leaves only ambient bits lowered. A step-5
// failure leaves the target effective/permitted/inheritable installed
// with CAP_SETPCAP removed again when the target drops it (applyTarget
// retries the step-6 capset before returning), and a bounding set
// reduced by the bits dropped before the failing one. A step-6 or
// step-7 failure leaves the bounding set complete and the masks
// described in the error. The error names the failing step and the
// state the thread was left in.
//
// When a failure occurs after the thread state has already been
// mutated, the OS thread pin is retained — the goroutine carries the
// lock away on exit. Releasing a thread whose capability state
// diverged mid-way would hand the scheduler a thread that behaves
// differently from its peers. Failures before the first mutation
// (snapshot read, validation) release the pin as usual. Same policy as
// the sibling seccomp package.
func applyTarget(target func(cur Sets) Sets) (retErr error) {
	runtime.LockOSThread()
	mutated := false
	defer func() {
		if retErr == nil || !mutated {
			runtime.UnlockOSThread()
		}
	}()

	cur, err := getSnapshot()
	if err != nil {
		return fmt.Errorf("applySnapshot: read current state: %w", err)
	}
	s := target(cur)
	if err = validateTarget(cur, s); err != nil {
		return err
	}

	lowered, c, err := forEachCap(cur.Ambient&^s.Ambient, ambientLower)
	mutated = lowered
	if err != nil {
		return fmt.Errorf(
			"applySnapshot: step 3 (ambient lower of %s) failed; "+
				"thread state: ambient bits up to but not including %s have been lowered, "+
				"capset/bounding/ambient-raise have NOT run: %w",
			c, c, err,
		)
	}

	// Keep CAP_SETPCAP for the bounding drops; validateTarget has made
	// sure it is in the current permitted set.
	toDrop := cur.Bounding &^ s.Bounding
	eff, perm := s.Effective, s.Permitted
	if toDrop != 0 {
		eff = setBit(eff, CAP_SETPCAP)
		perm = setBit(perm, CAP_SETPCAP)
	}
	if err = capset(eff, perm, s.Inheritable); err != nil {
		return fmt.Errorf(
			"applySnapshot: step 4 (capset eff=%#x perm=%#x inh=%#x) failed; "+
				"thread state: ambient lowered, capset rejected, "+
				"effective/permitted/inheritable/bounding still match the pre-call snapshot: %w",
			eff, perm, s.Inheritable, err,
		)
	}
	mutated = true

	if _, c, err = forEachCap(toDrop, boundingDrop); err != nil {
		dropErr := fmt.Errorf(
			"applySnapshot: step 5 (bounding drop of %s) failed; "+
				"thread state: requested effective/permitted/inheritable installed, "+
				"bounding bits up to but not including %s have been dropped, "+
				"every bit at %s and above is still in the bounding set: %w",
			c, c, c, err,
		)
		if eff == s.Effective && perm == s.Permitted {
			return dropErr
		}
		if revertErr := capset(s.Effective, s.Permitted, s.Inheritable); revertErr != nil {
			return errors.Join(dropErr, fmt.Errorf(
				"applySnapshot: removing CAP_SETPCAP kept for the bounding drop failed; "+
					"it is still effective and permitted: %w", revertErr))
		}
		return dropErr
	}

	if eff != s.Effective || perm != s.Permitted {
		if err = capset(s.Effective, s.Permitted, s.Inheritable); err != nil {
			return fmt.Errorf(
				"applySnapshot: step 6 (capset eff=%#x perm=%#x inh=%#x) failed; "+
					"thread state: bounding fully committed, CAP_SETPCAP kept for the drop "+
					"is still effective and permitted: %w",
				s.Effective, s.Permitted, s.Inheritable, err,
			)
		}
	}

	// The kernel requires a raised ambient bit to be in permitted ∩
	// inheritable, which the capset above has installed.
	if _, c, err = forEachCap(s.Ambient&^cur.Ambient, ambientRaise); err != nil {
		return fmt.Errorf(
			"applySnapshot: step 7 (ambient raise of %s) failed; "+
				"thread state: capset and bounding fully committed, "+
				"ambient bits up to but not including %s have been raised: %w",
			c, c, err,
		)
	}
	return nil
}

// forEachCap calls op for every cap in mask in ascending order and
// stops at the first error, returning the cap it failed on. done
// reports whether any call succeeded.
func forEachCap(mask uint64, op func(Cap) error) (done bool, failed Cap, err error) {
	for c := Cap(0); c <= capLastCap; c++ {
		if !has(mask, c) {
			continue
		}
		if err := op(c); err != nil {
			return done, c, err
		}
		done = true
	}
	return done, 0, nil
}

// validateTarget rejects, before any syscall, a target the kernel
// would refuse in the middle of the sequence. It checks the target
// itself, not the masks of the intermediate capset that keeps
// CAP_SETPCAP for the bounding drops: that capset passes whenever the
// target does, and the target is what the final capset installs.
func validateTarget(cur, s Sets) error {
	// The capset(2) rules of cap_capset in security/commoncap.c, against
	// the thread's state before the call. The bounding set limits only
	// new inheritable bits; permitted bits stay allowed outside it.
	if extra := s.Permitted &^ cur.Permitted; extra != 0 {
		return fmt.Errorf(
			"%w: permitted mask %#x adds %#x, which is not in the current permitted set %#x",
			ErrInvalidOption, s.Permitted, extra, cur.Permitted,
		)
	}
	if extra := s.Effective &^ s.Permitted; extra != 0 {
		return fmt.Errorf(
			"%w: effective mask %#x has %#x, which is not in the requested permitted mask %#x",
			ErrInvalidOption, s.Effective, extra, s.Permitted,
		)
	}
	if extra := s.Inheritable &^ (cur.Inheritable | cur.Bounding); extra != 0 {
		return fmt.Errorf(
			"%w: inheritable mask %#x adds %#x, which is in neither the current inheritable set %#x nor the bounding set %#x",
			ErrInvalidOption, s.Inheritable, extra, cur.Inheritable, cur.Bounding,
		)
	}
	if !has(cur.Effective, CAP_SETPCAP) {
		if extra := s.Inheritable &^ (cur.Inheritable | cur.Permitted); extra != 0 {
			return fmt.Errorf(
				"%w: inheritable mask %#x adds %#x, which is in neither the current inheritable set %#x "+
					"nor the permitted set %#x, and CAP_SETPCAP is not effective",
				ErrInvalidOption, s.Inheritable, extra, cur.Inheritable, cur.Permitted,
			)
		}
	}
	// capset clears ambient bits that leave permitted ∩ inheritable, and
	// PR_CAP_AMBIENT_RAISE refuses them.
	if extra := s.Ambient &^ (s.Permitted & s.Inheritable); extra != 0 {
		return fmt.Errorf(
			"%w: ambient mask %#x has %#x, which is not in both the requested permitted %#x and inheritable %#x masks",
			ErrInvalidOption, s.Ambient, extra, s.Permitted, s.Inheritable,
		)
	}
	// PR_CAPBSET_DROP needs CAP_SETPCAP in the effective set; it can
	// be made effective only from the permitted set.
	if toDrop := cur.Bounding &^ s.Bounding; toDrop != 0 && !has(cur.Permitted, CAP_SETPCAP) {
		return fmt.Errorf(
			"%w: dropping bounding bits %#x requires CAP_SETPCAP, which is not in the permitted set %#x",
			ErrInvalidOption, toDrop, cur.Permitted,
		)
	}
	return nil
}

// dropAllExcept is the Linux implementation of [DropAll] /
// [DropAllExcept]. It keeps each requested cap where the thread
// currently holds it — in bounding when it is in the bounding set, in
// effective and permitted when it is permitted — clears everything
// else, inheritable and ambient included, and delegates to
// [applyTarget]. A requested cap the thread does not hold is not an
// error: there is nothing to keep.
func dropAllExcept(keep []Cap) error {
	var mask uint64
	for _, c := range keep {
		if c < 0 || c > capLastCap {
			return fmt.Errorf("%w: %s is out of range", ErrInvalidOption, c)
		}
		mask = setBit(mask, c)
	}
	return applyTarget(func(cur Sets) Sets {
		perm := cur.Permitted & mask
		return Sets{
			Effective:   perm,
			Permitted:   perm,
			Inheritable: 0,
			Bounding:    cur.Bounding & mask,
			Ambient:     0,
		}
	})
}

// boundingDrop is the Linux implementation of [BoundingDrop]. Calling
// PR_CAPBSET_DROP on a cap not currently in the bounding set is a
// no-op and returns success, so the caller does not need to check
// first.
func boundingDrop(c Cap) error {
	if c < 0 || c > capLastCap {
		return fmt.Errorf("%w: %s is out of range", ErrInvalidOption, c)
	}
	if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(c), 0, 0, 0); err != nil {
		return fmt.Errorf("%w: prctl(PR_CAPBSET_DROP, %s): %w", ErrFailed, c, err)
	}
	return nil
}

// ambientClearAll is the Linux implementation of [AmbientClearAll].
func ambientClearAll() error {
	if err := unix.Prctl(
		unix.PR_CAP_AMBIENT,
		unix.PR_CAP_AMBIENT_CLEAR_ALL,
		0, 0, 0,
	); err != nil {
		return fmt.Errorf("%w: prctl(PR_CAP_AMBIENT_CLEAR_ALL): %w", ErrFailed, err)
	}
	return nil
}

// ambientRaise is the Linux implementation of [AmbientRaise].
func ambientRaise(c Cap) error {
	if c < 0 || c > capLastCap {
		return fmt.Errorf("%w: %s is out of range", ErrInvalidOption, c)
	}
	if err := unix.Prctl(
		unix.PR_CAP_AMBIENT,
		unix.PR_CAP_AMBIENT_RAISE,
		uintptr(c), 0, 0,
	); err != nil {
		return fmt.Errorf("%w: prctl(PR_CAP_AMBIENT_RAISE, %s): %w", ErrFailed, c, err)
	}
	return nil
}

// ambientLower is the Linux implementation of [AmbientLower].
func ambientLower(c Cap) error {
	if c < 0 || c > capLastCap {
		return fmt.Errorf("%w: %s is out of range", ErrInvalidOption, c)
	}
	if err := unix.Prctl(
		unix.PR_CAP_AMBIENT,
		unix.PR_CAP_AMBIENT_LOWER,
		uintptr(c), 0, 0,
	); err != nil {
		return fmt.Errorf("%w: prctl(PR_CAP_AMBIENT_LOWER, %s): %w", ErrFailed, c, err)
	}
	return nil
}
