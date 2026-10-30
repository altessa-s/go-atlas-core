// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package signals

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/altessa-s/go-atlas-core/runtime"
	"github.com/altessa-s/go-atlas-core/time/clock"

	corecontext "github.com/altessa-s/go-atlas-core/context"
	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// ContextHandler is the callback signature for signal handlers registered with
// [Signal.AddHandler] and related methods. The context carries the handler's
// timeout deadline (see [WithHandlerTimeout]) and is derived from the [Signal]'s
// global context, so it is also canceled during [Signal.Shutdown].
//
// Returning a non-nil error causes the [ErrorHandler] to be invoked. Panics
// within a ContextHandler are recovered and reported as [*PanicError].
//
// For handlers that do not need context, simply ignore the parameter:
//
//	handler := func(_ context.Context, sig os.Signal) error {
//	    // Handle signal
//	    return nil
//	}
type ContextHandler func(ctx context.Context, sig os.Signal) error

// Priority defines the execution order of signal handlers. Higher numeric
// values execute before lower ones. Use the predefined constants
// ([PriorityLowest] through [PriorityHighest]) for common levels.
type Priority int

// Predefined [Priority] levels for signal handlers, ranging from 1 (lowest)
// to 100 (highest). Any other value is a valid custom priority: it is ordered
// relative to the predefined levels, and handlers sharing a priority run in
// registration order.
const (
	// PriorityLowest (1) is suitable for non-critical handlers such as logging
	// and metrics collection.
	PriorityLowest Priority = 1

	// PriorityLow (25) is suitable for background tasks and deferred cleanup.
	PriorityLow Priority = 25

	// PriorityNormal (50) is the default priority assigned by [Signal.AddHandler].
	PriorityNormal Priority = 50

	// PriorityHigh (75) is suitable for important business-logic handlers that
	// should run before cleanup but after critical system operations.
	PriorityHigh Priority = 75

	// PriorityHighest (100) is reserved for critical system handlers such as
	// graceful shutdown coordinators and security-related operations.
	PriorityHighest Priority = 100
)

// Internal performance tuning constants.
const (
	// HandlerSliceInitialCapacity is the pre-allocated capacity for the
	// handler slice collected before execution, reducing append allocations.
	HandlerSliceInitialCapacity = 32

	// PriorityBucketCount is the number of pre-allocated priority buckets,
	// one per predefined [Priority] level, used by the priority queue
	// optimization for large handler sets. Custom priorities get their own
	// buckets on demand.
	PriorityBucketCount = 5
)

// handlerEntry represents a single handler with its execution mode and timeout configuration.
// It encapsulates either a regular Handler or a ContextHandler along with execution metadata.
type handlerEntry struct {
	// contextHandler is the context-aware signal handler function
	contextHandler ContextHandler
	// timeout specifies the maximum execution time for this handler
	timeout time.Duration
	// priority defines the execution order (higher numbers execute first)
	priority Priority
}

// Signal is a signal handler that provides a comprehensive signal processing system.
// It supports both signal-specific handlers and broadcast handlers that respond to all signals.
//
// Key features:
//   - Concurrent signal processing with configurable worker pool
//   - Context-aware handlers with timeout support
//   - Comprehensive error handling with panic recovery
//   - Graceful shutdown with configurable timeouts
//   - Thread-safe operations for concurrent use
//
// The Signal struct manages the entire lifecycle of signal handling from registration
// to processing to shutdown, ensuring reliable and predictable signal processing behavior.
type Signal struct {
	// stop is used to signal shutdown to the background listener goroutine
	stop chan struct{}
	// wg tracks active goroutines for graceful shutdown
	wg *sync.WaitGroup
	// signalChannel receives OS signals from the signal package
	signalChannel chan os.Signal
	// handlers maps specific OS signals to their registered handlers
	handlers map[os.Signal][]handlerEntry
	// signals contains the list of OS signals this handler listens for
	signals []os.Signal
	// broadcastHandlers contains handlers that are executed for every signal
	broadcastHandlers []handlerEntry
	// mx protects concurrent access to handlers and broadcastHandlers
	mx sync.RWMutex
	// started tracks whether the signal handler has been started (atomic for thread safety)
	started *atomic.Bool
	// workerPool limits the number of concurrent handler executions
	workerPool chan struct{}
	// errorHandler is called when handlers return errors or panic
	errorHandler ErrorHandler
	// shutdownTimeout defines the maximum time to wait for graceful shutdown
	shutdownTimeout time.Duration
	// handlerTimeout defines the default timeout for individual handler execution
	handlerTimeout time.Duration
	// shutdownHooks runs the shutdown hooks at the end of Shutdown
	shutdownHooks runtime.ShutdownHook
	// clock drives the parallel-mode grace period after a shutdown request
	clock clock.Clock

	// executionMode determines how handlers are executed (sequential vs parallel)
	executionMode ExecutionMode
	// globalCtx is used for coordinated shutdown of all handlers
	globalCtx context.Context
	// globalCancel cancels the global context during shutdown
	globalCancel context.CancelFunc
}

// New creates a new Signal handler instance with the specified configuration options.
//
// The signal handler is created with sensible defaults that can be overridden using functional options:
//   - Worker pool size: 10 concurrent workers
//   - Shutdown timeout: 30 seconds for graceful shutdown
//   - Handler timeout: 5 seconds per individual handler execution
//   - Signal channel buffer: 1 (prevents signal loss)
//   - Stop channel buffer: 1 (ensures shutdown signal delivery)
//
// Available options:
//   - WithSignals: specify which OS signals to listen for
//   - WithWorkerPoolSize: configure concurrent handler execution limit
//   - WithHandlerTimeout: set timeout for individual handler execution
//   - WithShutdownTimeout: set timeout for graceful shutdown
//   - WithErrorHandler: configure custom error handling
//   - WithShutdownHooks: replace the hooks run at the end of Shutdown
//   - WithClock: inject the clock driving the parallel-mode grace period
//
// Example:
//
//	handler := signals.New(
//	    signals.WithSignals(syscall.SIGTERM, syscall.SIGINT),
//	    signals.WithWorkerPoolSize(20),
//	    signals.WithHandlerTimeout(10*time.Second),
//	)
//
// The returned Signal instance is ready to have handlers registered and can be started
// immediately. All operations on the Signal are thread-safe.
func New(opts ...Option) *Signal {
	o := newOptions(opts...)

	sig := &Signal{
		handlers:          map[os.Signal][]handlerEntry{},
		broadcastHandlers: []handlerEntry{},
		signalChannel:     make(chan os.Signal, o.signalChannelBuffer),
		wg:                &sync.WaitGroup{},
		stop:              make(chan struct{}, DefaultStopChannelBuffer),
		started:           &atomic.Bool{},
		workerPool:        make(chan struct{}, o.workerPoolSize),
		shutdownTimeout:   o.shutdownTimeout,
		handlerTimeout:    o.handlerTimeout,
		executionMode:     o.executionMode,
		signals:           o.signals,
		errorHandler:      o.errorHandler,
		shutdownHooks:     o.shutdownHooks,
		clock:             o.clock,
	}

	// Create global context for coordinated shutdown
	sig.globalCtx, sig.globalCancel = context.WithCancel(context.Background())

	return sig
}

// newHandlerEntry creates a handlerEntry for a context-aware signal handler.
// The entry is configured with the signal handler's default timeout and normal priority.
// This is an internal helper method used by AddHandler.
func (s *Signal) newHandlerEntry(handler ContextHandler) handlerEntry {
	return handlerEntry{
		contextHandler: handler,
		timeout:        s.handlerTimeout,
		priority:       PriorityNormal,
	}
}

// newPriorityHandlerEntry creates a handlerEntry for a context handler with custom priority.
// This is an internal helper method used by AddHandlerWithPriority.
func (s *Signal) newPriorityHandlerEntry(handler ContextHandler, priority Priority) handlerEntry {
	return handlerEntry{
		contextHandler: handler,
		timeout:        s.handlerTimeout,
		priority:       priority,
	}
}

// addSignalHandler is an internal helper method that safely adds a handler entry
// to the handlers map for the specified signals. It handles map initialization
// and ensures thread-safe access through mutex locking.
func (s *Signal) addSignalHandler(entry handlerEntry, signals ...os.Signal) {
	s.mx.Lock()
	defer s.mx.Unlock()

	for _, sig := range signals {
		if _, ok := s.handlers[sig]; !ok {
			s.handlers[sig] = make([]handlerEntry, 0)
		}
		s.handlers[sig] = append(s.handlers[sig], entry)
	}
}

// addBroadcastHandler is an internal helper method that safely adds a handler entry
// to the broadcast handlers slice. Broadcast handlers are executed for every signal
// regardless of the signal type. This method ensures thread-safe access through mutex locking.
func (s *Signal) addBroadcastHandler(entry handlerEntry) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.broadcastHandlers = append(s.broadcastHandlers, entry)
}

// AddHandler registers hdlr to be called at [PriorityNormal] when any of the
// specified signals are received. Multiple handlers may be registered for the
// same signal. A nil hdlr is silently ignored. The method returns s for
// method chaining. It is safe to call from multiple goroutines.
func (s *Signal) AddHandler(hdlr ContextHandler, signals ...os.Signal) *Signal {
	if hdlr == nil {
		return s
	}

	s.addSignalHandler(s.newHandlerEntry(hdlr), signals...)
	return s
}

// AddBroadcastHandler registers h to be called at [PriorityNormal] for every
// signal received, regardless of signal type. A nil h is silently ignored.
// The method returns s for method chaining. It is safe to call from multiple
// goroutines.
func (s *Signal) AddBroadcastHandler(h ContextHandler) *Signal {
	if h == nil {
		return s
	}

	s.addBroadcastHandler(s.newHandlerEntry(h))
	return s
}

// AddHandlerWithPriority registers hdlr with a custom [Priority] for the
// specified signals. Higher priority handlers execute before lower priority
// ones (in [SequentialMode]) or start sooner (in [ParallelMode]). A nil hdlr
// is silently ignored. The method returns s for method chaining. It is safe
// to call from multiple goroutines.
//
// Example:
//
//	// Critical shutdown handler executes first
//	handler.AddHandlerWithPriority(shutdownHandler, signals.PriorityHighest, syscall.SIGTERM)
//
//	// Normal cleanup handler executes after
//	handler.AddHandlerWithPriority(cleanupHandler, signals.PriorityNormal, syscall.SIGTERM)
func (s *Signal) AddHandlerWithPriority(hdlr ContextHandler, priority Priority, signals ...os.Signal) *Signal {
	if hdlr == nil {
		return s
	}

	s.addSignalHandler(s.newPriorityHandlerEntry(hdlr, priority), signals...)
	return s
}

// AddBroadcastHandlerWithPriority registers h with a custom [Priority] for
// every signal. A nil h is silently ignored. The method returns s for method
// chaining. It is safe to call from multiple goroutines.
//
// Example:
//
//	// Critical logging executes first for all signals
//	handler.AddBroadcastHandlerWithPriority(criticalLogger, signals.PriorityHighest)
func (s *Signal) AddBroadcastHandlerWithPriority(h ContextHandler, priority Priority) *Signal {
	if h == nil {
		return s
	}

	s.addBroadcastHandler(s.newPriorityHandlerEntry(h, priority))
	return s
}

// Start begins listening for the configured OS signals and dispatching them to
// registered handlers. It is idempotent; calling it more than once has no
// effect. The method returns s for method chaining.
func (s *Signal) Start() *Signal {
	if !s.started.CompareAndSwap(false, true) {
		return s
	}

	// Start the listener goroutine first to avoid race condition
	s.wg.Go(s.signalsListener)

	// Then start receiving signals - goroutine is already running
	signal.Notify(s.signalChannel, s.signals...)
	return s
}

// Stop is a convenience method that calls [Signal.Shutdown] with
// [context.Background]. It blocks until all in-flight handlers complete or
// the shutdown timeout expires.
func (s *Signal) Stop() error {
	return s.Shutdown(context.Background())
}

// Shutdown gracefully stops the signal handler. It immediately stops
// listening for new OS signals, then waits for all in-flight handlers to
// complete, and finally runs the shutdown hooks (see [WithShutdownHooks];
// the process-wide [runtime.RunShutdownHooks] by default). If ctx is
// canceled or times out before completion, the handler context is canceled,
// shutdown hooks are still attempted on a best-effort basis and a wrapped
// context error is returned. Handlers that ignore cancellation of their
// context keep running after Shutdown returns on timeout: their goroutines
// are not reclaimed, and a later [Signal.Wait] blocks until they exit.
// Passing a nil ctx returns an error. Shutdown is idempotent; subsequent
// calls after the first return nil.
func (s *Signal) Shutdown(ctx context.Context) error {
	// Validated before the state change: rejecting a nil ctx after marking
	// the handler stopped would leave the listener running with no way to
	// stop it.
	if ctx == nil {
		return fmt.Errorf("shutdown: ctx cannot be nil")
	}

	if !s.started.CompareAndSwap(true, false) {
		return nil // Already stopped
	}

	// If caller didn't set a deadline, apply the configured shutdown timeout.
	ctx, cancel := corecontext.ApplyTimeout(ctx, s.shutdownTimeout)
	defer cancel()

	// Stop receiving new OS signals immediately.
	signal.Stop(s.signalChannel)

	// Signal shutdown to listener
	select {
	case s.stop <- struct{}{}:
	default:
		// Stop signal already queued.
	}

	// Wait for graceful completion
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed successfully
		// Run global shutdown hooks
		if err := s.shutdownHooks(ctx); err != nil {
			return err
		}
		s.globalCancel() // Cancel context after graceful completion
		return nil
	case <-ctx.Done():
		// Timeout occurred: cancel the handler context FIRST so in-flight
		// handlers (and the wg.Wait waiter goroutine above) unwind
		// immediately instead of running on while hooks execute.
		s.globalCancel()
		// Still try to run hooks, best effort — the original ctx is expired.
		s.shutdownHooks(context.Background()) //nolint:errcheck,contextcheck // best-effort during shutdown timeout, original ctx is expired
		return coreerrs.Wrap(ctx.Err(), "shutdown timeout")
	}
}

// Wait blocks the calling goroutine until the signal handler has stopped,
// either via [Signal.Stop], [Signal.Shutdown], or because all background
// goroutines have exited. It is typically called after [Signal.Start] to keep
// the main goroutine alive.
func (s *Signal) Wait() {
	s.wg.Wait()
}

// signalsListener is the core background goroutine that listens for OS signals and
// dispatches them to registered handlers. It implements a worker pool pattern to
// limit the number of concurrent handler executions, preventing resource exhaustion.
//
// The listener operates as follows:
//   - Waits for signals on s.signalChannel
//   - Attempts to acquire a worker slot from s.workerPool
//   - If a slot is available, launches a goroutine to handle the signal
//   - If no slots are available, drops the signal and reports an error
//   - Shuts down gracefully when receiving on s.stop channel
//
// This method should only be called once and runs until Stop() or Shutdown() is called.
func (s *Signal) signalsListener() {
	for {
		select {
		case sig := <-s.signalChannel:
			// Use worker pool to limit concurrent handlers
			select {
			case s.workerPool <- struct{}{}:
				s.wg.Go(func() {
					defer func() { <-s.workerPool }()
					s.runHandlers(sig)
				})
			default:
				// Worker pool is full, handle error
				s.safeCallErrorHandler(sig, errors.New("worker pool is full, signal handler dropped"))
			}
		case <-s.stop:
			// Stop listening. signal.Stop is owned by Shutdown, which has
			// already called it before queueing the stop message — calling
			// it again here would be harmless (the stdlib serializes it)
			// but obscures who is responsible for unsubscribing.
			return
		}
	}
}

// runHandlers orchestrates the execution of all registered handlers for a specific signal.
// Uses optimized execution path with priority queue optimizations.
func (s *Signal) runHandlers(sig os.Signal) {
	// Count and snapshot under a single read lock: releasing it between
	// the count and the copy would let a concurrent AddHandler change the
	// set, so the branch decision below could be made on a stale total.
	s.mx.RLock()
	sigHandlers := s.handlers[sig]
	totalHandlers := len(s.broadcastHandlers) + len(sigHandlers)

	if totalHandlers == 0 {
		s.mx.RUnlock()
		return
	}

	var handlers []handlerEntry

	// Use priority queue for large handler sets (>50 handlers for O(1) benefit)
	if totalHandlers > LargeHandlerCountThreshold {
		pq := newPriorityQueue()

		for _, entry := range s.broadcastHandlers {
			pq.addHandler(entry)
		}
		for _, entry := range sigHandlers {
			pq.addHandler(entry)
		}
		s.mx.RUnlock()

		handlers = pq.getHandlersInOrder()
	} else {
		// Use traditional approach for smaller handler sets
		handlers = make([]handlerEntry, 0, HandlerSliceInitialCapacity)
		handlers = append(handlers, s.broadcastHandlers...)
		handlers = append(handlers, sigHandlers...)
		s.mx.RUnlock()

		s.sortHandlers(handlers)
	}

	// Execute handlers according to configured mode
	switch s.executionMode {
	case ParallelMode:
		s.runHandlersParallelOptimized(handlers, sig)
	case SequentialMode:
		s.runHandlersSequentialOptimized(handlers, sig)
	}
}

// runHandlersSequentialOptimized executes handlers sequentially with optimizations
func (s *Signal) runHandlersSequentialOptimized(handlers []handlerEntry, sig os.Signal) {
	for _, entry := range handlers {
		// Quick check for shutdown without timeout handlers
		if entry.timeout > 0 {
			select {
			case <-s.globalCtx.Done():
				return // Shutdown requested
			default:
			}
		}

		s.executeHandlerEntryOptimized(entry, sig)
	}
}

// runHandlersParallelOptimized executes handlers in parallel with global goroutine pool limiting.
//
// The pool is hand-rolled rather than built on core/runtime/concurrency:
// workerPool is a process-wide budget shared by every signal dispatch, and a
// handler that finds it full runs synchronously on the caller instead of
// waiting for a slot. Process bounds a single call and always blocks for a
// slot, so it cannot express the shared budget or the non-blocking fallback.
func (s *Signal) runHandlersParallelOptimized(handlers []handlerEntry, sig os.Signal) {
	if len(handlers) == 0 {
		return
	}

	// Use a WaitGroup to wait for all handlers to complete
	var wg sync.WaitGroup

	// Launch each handler using worker pool
	for _, entry := range handlers {
		// Try to acquire slot from worker pool (non-blocking)
		select {
		case s.workerPool <- struct{}{}:
			// Got slot, launch handler
			wg.Go(func() {
				defer func() { <-s.workerPool }() // Release worker pool slot

				// Check shutdown status
				select {
				case <-s.globalCtx.Done():
					return // Shutdown requested, don't execute handler
				default:
				}

				s.executeHandlerEntryOptimized(entry, sig)
			})

		default:
			// Global pool is full, execute synchronously to avoid goroutine explosion
			// This prevents exceeding the configured global handler pool limit

			// Check shutdown status
			select {
			case <-s.globalCtx.Done():
				continue // Shutdown requested, skip this handler
			default:
			}

			// Execute handler synchronously (no new goroutine)
			s.executeHandlerEntryOptimized(entry, sig)
		}
	}

	// Wait for completion or shutdown
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All handlers completed
	case <-s.globalCtx.Done():
		// Shutdown requested - give handlers a moment to finish gracefully.
		// globalCtx is already done, so the grace sleep runs on a context
		// detached from it and is canceled once either branch below fires.
		graceCtx, cancel := context.WithCancel(context.WithoutCancel(s.globalCtx))
		defer cancel()
		graceOver := make(chan struct{})
		go func() {
			defer close(graceOver)
			_ = s.clock.Sleep(graceCtx, ResponsiveTimeoutDuration)
		}()

		select {
		case <-done:
			// Completed during grace period
		case <-graceOver:
		}
	}
}

// executeHandlerEntryOptimized provides optimized handler execution with zero-timeout fast path
func (s *Signal) executeHandlerEntryOptimized(entry handlerEntry, sig os.Signal) {
	defer func() {
		if r := recover(); r != nil {
			s.safeCallErrorHandler(sig, &PanicError{
				Signal: sig,
				Panic:  r,
			})
		}
	}()

	// For zero timeout, execute synchronously without any overhead
	if entry.timeout <= 0 {
		err := entry.contextHandler(s.globalCtx, sig)
		if err != nil {
			s.safeCallErrorHandler(sig, err)
		}
		return
	}

	// Create context with timeout - simple and direct
	ctx, cancel := corecontext.WithMaxTimeout(s.globalCtx, entry.timeout)
	defer cancel()

	done := make(chan error, 1)
	// Track the handler goroutine in s.wg so Shutdown's wg.Wait cannot return
	// while a handler is still running after this function returns on timeout.
	// The parent's deferred cancel signals ctx so a well-behaved handler exits.
	s.wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &PanicError{Signal: sig, Panic: r}
			}
		}()

		err := entry.contextHandler(ctx, sig)
		done <- err
	})

	select {
	case err := <-done:
		if err != nil {
			s.safeCallErrorHandler(sig, err)
		}
	case <-ctx.Done():
		// Check if it's a timeout or shutdown
		select {
		case <-s.globalCtx.Done():
			// Global shutdown, don't report as timeout
			return
		default:
			// Handler timeout
			timeoutErr := &TimeoutError{
				Signal:  sig,
				Timeout: entry.timeout,
			}
			s.safeCallErrorHandler(sig, timeoutErr)
		}
	}
}
