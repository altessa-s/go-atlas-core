// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrOverloaded means the group cannot admit another distinct key. Existing
// computations can still be joined while the group is at capacity.
var ErrOverloaded = errors.New("singleflight: concurrent computation limit reached")

// ErrAborted means a callback exited with runtime.Goexit instead of returning.
var ErrAborted = errors.New("singleflight: callback exited without returning")

// ErrClosed means Close or Wait has stopped admission, including joins to existing keys.
var ErrClosed = errors.New("singleflight: group closed")

type lifecycleWaiters struct {
	count  int
	cancel context.CancelFunc
}

type call[V any] struct {
	lifecycle  *lifecycleWaiters
	joined     bool
	done       chan struct{}
	value      V
	err        error
	panicValue any
}

// Group shares one computation among callers with equal keys. Key equality is
// Go equality, never string formatting. Create a group with New.
//
// Each caller can stop waiting independently. Shared work retains the first
// caller's context values and always uses the group's timeout. By default,
// caller cancellation does not cancel shared work. WithLifecycle additionally
// cancels work when its parent ends or the last waiter cancels; the latter also
// detaches the call so later callers can start fresh subject to capacity.
// Callbacks must honor their context. Capacity is released only when the callback
// exits, even if every caller has stopped waiting. Callback panics are re-raised
// in waiting callers unless WithPanicAsError supplies a non-nil replacement error.
type Group[K comparable, V any] struct {
	mu            sync.Mutex
	wg            sync.WaitGroup
	closed        bool
	active        int
	lifecycle     context.Context
	panicAsError  error
	calls         map[K]*call[V]
	maxConcurrent int
	timeout       time.Duration
}

// New creates a group. Non-positive limits allow unlimited distinct work;
// non-positive timeout options retain the default deadline.
func New[K comparable, V any](opts ...Option) *Group[K, V] {
	o := newOptions(opts...)
	return &Group[K, V]{calls: make(map[K]*call[V]), maxConcurrent: o.maxConcurrent, timeout: o.timeout, lifecycle: o.lifecycle, panicAsError: o.panicAsError}
}

// Do computes fn once per in-flight key, or joins an existing computation.
// There is no background queue: a new key at capacity returns ErrOverloaded.
// Keys unequal to themselves (such as NaN) run independently without deduplication,
// but still count toward capacity and Wait.
// Callers wait independently; WithLifecycle enables parent and last-waiter
// cancellation of shared work without disabling the group timeout.
// Callback panics are re-raised in callers still waiting for the result unless
// WithPanicAsError supplies a non-nil error, which is returned without the panic value.
func (g *Group[K, V]) Do(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	v, _, err := g.DoShared(ctx, key, fn)
	return v, err
}

// DoShared is Do with a flag reporting whether another caller joined the work.
// A true flag applies to every successful waiter, even if another waiter canceled.
// Canceled and rejected callers return false.
func (g *Group[K, V]) DoShared(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, bool, error) {
	var zero V
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return zero, false, ErrClosed
	}
	c, exists := g.calls[key]
	if !exists {
		if g.maxConcurrent > 0 && g.active >= g.maxConcurrent {
			g.mu.Unlock()
			return zero, false, ErrOverloaded
		}
		c = &call[V]{done: make(chan struct{})}
		work := context.WithoutCancel(ctx)
		if g.lifecycle != nil {
			var cancel context.CancelFunc
			work, cancel = context.WithCancel(work)
			c.lifecycle = &lifecycleWaiters{cancel: cancel}
		}
		// Non-reflexive keys cannot be retrieved or deleted from a map.
		if key == key { //nolint:gocritic // Self-comparison detects NaN, including within composite keys.
			g.calls[key] = c
		}
		g.active++
		g.wg.Add(1)
		go g.run(work, key, c, fn)
	} else {
		c.joined = true
	}
	if c.lifecycle != nil {
		c.lifecycle.count++
	}
	g.mu.Unlock()
	select {
	case <-ctx.Done():
		if g.lifecycle != nil {
			g.mu.Lock()
			c.lifecycle.count--
			if c.lifecycle.count == 0 {
				c.lifecycle.cancel()
				if g.calls[key] == c {
					delete(g.calls, key)
				}
			}
			g.mu.Unlock()
		}
		return zero, false, ctx.Err()
	case <-c.done:
		if c.panicValue != nil {
			panic(c.panicValue)
		}
		return c.value, c.joined, c.err
	}
}

// Forget detaches key so later callers start fresh. Existing work and waiters
// continue, and detached work still counts against the concurrency limit.
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
}

// Close stops admission without canceling existing work. It is idempotent.
func (g *Group[K, V]) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// Wait closes admission and waits for all work, including forgotten calls.
// It implies Close and is safe to call concurrently with Do and other Wait calls.
// It does not cancel work; callbacks must honor cancellation or finish.
func (g *Group[K, V]) Wait() {
	g.Close()
	g.wg.Wait()
}

func (g *Group[K, V]) run(ctx context.Context, key K, c *call[V], fn func(context.Context) (V, error)) {
	defer g.wg.Done()
	if g.lifecycle != nil {
		stop := context.AfterFunc(g.lifecycle, c.lifecycle.cancel) //nolint:contextcheck // Explicit group lifecycle supplements caller values.
		defer stop()
		defer c.lifecycle.cancel()
		if g.lifecycle.Err() != nil {
			c.lifecycle.cancel()
		}
	}
	returned := false
	defer func() {
		p := recover()
		if p != nil {
			if g.panicAsError != nil {
				c.err = g.panicAsError
			} else {
				c.panicValue = p
			}
		}
		if !returned && p == nil {
			c.err = ErrAborted
		}
		g.mu.Lock()
		if g.calls[key] == c {
			delete(g.calls, key)
		}
		g.active--
		close(c.done)
		g.mu.Unlock()
	}()
	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	c.value, c.err = fn(ctx)
	returned = true
}
