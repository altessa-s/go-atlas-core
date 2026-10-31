// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package cycle

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/altessa-s/go-atlas-core/runtime/panics"
)

// ErrManaged means an external driver owns the cycle.
var ErrManaged = errors.New("cycle: managed, direct calls not allowed")

// Guard wraps one cycle function. Construct it with New; do not copy after use.
// All methods are safe for concurrent use. Overlapping calls return nil immediately.
type Guard struct {
	fn      func(context.Context) error
	managed atomic.Bool
	running atomic.Bool
}

// New binds fn to a guard. A nil function panics.
func New(fn func(context.Context) error) *Guard {
	panics.Must(fn != nil, "cycle: nil function")
	return &Guard{fn: fn}
}

// Managed permanently hands the cycle to an external driver and returns its entry
// point. It is a convenience for MarkManaged followed by Entry. For fallible
// registration, obtain Entry first and call MarkManaged only after success.
func (g *Guard) Managed() func(context.Context) error {
	g.MarkManaged()
	return g.Entry()
}

// Entry returns the guarded entry point without marking the cycle managed.
// Register this function, then call MarkManaged only if registration succeeds.
func (g *Guard) Entry() func(context.Context) error { return g.TryRun }

// MarkManaged permanently marks the cycle externally driven. The owner calls
// it after successful registration. A previously admitted manual run may finish.
func (g *Guard) MarkManaged() { g.managed.Store(true) }

// IsManaged reports whether the cycle has been marked externally driven.
func (g *Guard) IsManaged() bool { return g.managed.Load() }

// Run invokes the cycle unless MarkManaged or Managed has been called. An overlapping call
// returns nil without waiting or observing the running call's error.
func (g *Guard) Run(ctx context.Context) error {
	if g.managed.Load() {
		return ErrManaged
	}
	return g.TryRun(ctx)
}

// TryRun invokes the bound function without checking managed ownership. Internal
// drivers use it to share the overlap guard with the external entry point.
func (g *Guard) TryRun(ctx context.Context) error { return g.TryRunFunc(ctx, g.fn) }

// TryRunFunc invokes fn under the same overlap guard, without checking managed
// ownership or replacing the bound function. This supports per-call closures
// that capture additional results. Overlaps return nil without invoking fn.
// A nil fn panics if admitted, and panics always release the overlap guard.
func (g *Guard) TryRunFunc(ctx context.Context, fn func(context.Context) error) error {
	if !g.running.CompareAndSwap(false, true) {
		return nil
	}
	defer g.running.Store(false)
	return fn(ctx)
}
