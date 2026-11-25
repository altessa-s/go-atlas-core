// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// ErrShutdownHookPanic indicates that a shutdown hook panicked.
var ErrShutdownHookPanic = errors.New("shutdown hook panicked")

// UnfinishedHooksError reports which hooks had not finished when a shutdown
// caller's context ended. Hooks are identified by their one-based registration
// positions, from 1 through Remaining.
type UnfinishedHooksError struct {
	Remaining int
	cause     error
}

// Error describes the unfinished hooks and why the wait ended.
func (e *UnfinishedHooksError) Error() string {
	return fmt.Sprintf("shutdown hooks 1 through %d did not finish: %v", e.Remaining, e.cause)
}

// Unwrap returns the context error that ended the wait.
func (e *UnfinishedHooksError) Unwrap() error { return e.cause }

// ShutdownHook is a function executed during application shutdown via
// [RunShutdownHooks] or [HookGroup.Shutdown]. The provided context may carry a
// deadline; implementations should respect cancellation. Returning a non-nil
// error does not prevent subsequent hooks from running.
type ShutdownHook func(ctx context.Context) error

// HookGroup is an independently runnable set of shutdown hooks.
//
// The process-wide hooks reached through [OnShutdown] and [RunShutdownHooks]
// live in one implicit group whose lifetime is the process: it runs at most
// once, and a component that registered there cannot be stopped on its own.
// That is the right default for a resource that lives as long as the program,
// and the wrong one for a background component owned by a factory, a test, or
// a subsystem that is torn down and rebuilt — those need a scope of their own,
// which is what a HookGroup is.
//
// The zero value is ready to use. A group is safe for concurrent use.
//
// Example — a subsystem that owns a background component and can be stopped
// without stopping the process:
//
//	var hooks runtime.HookGroup
//	hooks.OnShutdown(engine.Stop)
//	hooks.OnShutdown(storage.Close)
//	...
//	if err := hooks.Shutdown(ctx); err != nil { // storage.Close, then engine.Stop
//	    return err
//	}
type HookGroup struct {
	mu        sync.Mutex
	hooks     []ShutdownHook
	done      chan struct{}
	remaining int
	err       error
}

// OnShutdown registers a hook to run when [HookGroup.Shutdown] is invoked.
// Hooks run in reverse registration order (LIFO), so resources registered
// first are cleaned up last. Safe for concurrent use.
func (g *HookGroup) OnShutdown(hook ShutdownHook) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.hooks = append(g.hooks, hook)
}

// Shutdown runs the group's hooks in LIFO order at most once. The first caller's
// context controls the sequence. When it ends, the active hook may continue
// running. Hooks that have yet to be scheduled are skipped. The result includes
// an [UnfinishedHooksError] if any hooks did not finish.
//
// A hook panic becomes [ErrShutdownHookPanic]. Errors are joined and returned,
// and a failing hook does not prevent subsequent hooks from running.
//
// Concurrent callers can stop waiting using their own contexts. Once the
// sequence ends, subsequent calls return its cached result. Hooks registered
// after the sequence starts are skipped.
func (g *HookGroup) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	if g.done != nil {
		done := g.done
		g.mu.Unlock()

		select {
		case <-done:
			return g.err
		case <-ctx.Done():
			g.mu.Lock()
			defer g.mu.Unlock()

			select {
			case <-done:
				return g.err
			default:
				if g.remaining == 0 {
					return ctx.Err()
				}

				return &UnfinishedHooksError{Remaining: g.remaining, cause: ctx.Err()}
			}
		}
	}

	g.done = make(chan struct{})
	hooks := g.hooks
	g.remaining = len(hooks)
	g.mu.Unlock()

	var errs []error

	for remaining := len(hooks); remaining > 0; remaining-- {
		err := ctx.Err()
		finished := false

		if err == nil {
			err, finished = waitShutdownHook(ctx, hooks[remaining-1])
		}

		if !finished {
			errs = append(errs, &UnfinishedHooksError{Remaining: remaining, cause: err})
			break
		}

		if err != nil {
			errs = append(errs, err)
		}

		g.mu.Lock()
		g.remaining = remaining - 1
		g.mu.Unlock()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.err = errors.Join(errs...)
	close(g.done)

	return g.err
}

func waitShutdownHook(ctx context.Context, hook ShutdownHook) (error, bool) {
	result := make(chan error, 1)

	go func() {
		// The buffer lets the hook report completion after its caller leaves.
		err := ErrShutdownHookPanic

		defer func() {
			_ = recover()

			result <- err
		}()

		err = coreerrs.Wrap(hook(ctx), "shutdown hook failed")
	}()

	select {
	case err := <-result:
		return err, true
	case <-ctx.Done():
		// Retain the result if the hook completed as cancellation arrived.
		select {
		case err := <-result:
			return err, true
		default:
			return ctx.Err(), false
		}
	}
}

// processHooks is the implicit group behind [OnShutdown] and
// [RunShutdownHooks]: the hooks whose scope is the process itself.
var processHooks HookGroup

// OnShutdown registers a [ShutdownHook] to be called when [RunShutdownHooks] is invoked.
// Hooks are executed in reverse registration order (LIFO), so resources registered
// first are cleaned up last. This is safe to call from multiple goroutines concurrently.
//
// Use a [HookGroup] instead when the component's lifetime is shorter than the
// process's — a hook registered here can never be run on its own.
func OnShutdown(hook ShutdownHook) {
	processHooks.OnShutdown(hook)
}

// RunShutdownHooks executes the process-wide hooks registered via [OnShutdown].
// It has the same ordering, cancellation, and error behavior as [HookGroup.Shutdown].
func RunShutdownHooks(ctx context.Context) error {
	return processHooks.Shutdown(ctx)
}
