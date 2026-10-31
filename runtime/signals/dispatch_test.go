// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package signals

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/time/clock"
)

// The tests in this file drive the dispatch path (runHandlers and below)
// directly instead of delivering OS signals, so they run in synctest bubbles
// and in parallel without touching process-wide signal state.

// errorRecorder collects the errors passed to an ErrorHandler.
type errorRecorder struct {
	mu   sync.Mutex
	errs []error
	sigs []os.Signal
}

func (r *errorRecorder) handle(sig os.Signal, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
	r.sigs = append(r.sigs, sig)
}

func (r *errorRecorder) snapshot() ([]error, []os.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.errs), slices.Clone(r.sigs)
}

// orderRecorder records the ids of the handlers that ran, in run order.
type orderRecorder struct {
	mu  sync.Mutex
	ids []int
}

func (r *orderRecorder) handler(id int) ContextHandler {
	return func(context.Context, os.Signal) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.ids = append(r.ids, id)
		return nil
	}
}

func (r *orderRecorder) snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.ids)
}

func TestRunHandlers_NoHandlersIsNoop(t *testing.T) {
	t.Parallel()

	rec := &errorRecorder{}
	s := New(WithErrorHandler(rec.handle))
	s.AddHandler(func(context.Context, os.Signal) error {
		return errors.New("must not run")
	}, syscall.SIGUSR2)

	s.runHandlers(syscall.SIGUSR1)

	errs, _ := rec.snapshot()
	require.Empty(t, errs)
}

func TestRunHandlers_SequentialOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		priorities []Priority // one handler per entry, id = index
		broadcast  map[int]bool
		want       []int
	}{
		{
			name:       "broadcast and signal handlers merge by priority",
			priorities: []Priority{PriorityLow, PriorityHighest, PriorityNormal},
			broadcast:  map[int]bool{1: true},
			want:       []int{1, 2, 0},
		},
		{
			name:       "equal priority keeps registration order",
			priorities: []Priority{PriorityNormal, PriorityNormal, PriorityHigh, PriorityNormal},
			broadcast:  map[int]bool{},
			want:       []int{2, 0, 1, 3},
		},
		{
			// Broadcast handlers are collected before signal handlers, so a
			// broadcast handler wins a priority tie against a signal handler
			// registered earlier.
			name:       "broadcast wins ties against signal handlers",
			priorities: []Priority{PriorityNormal, PriorityNormal},
			broadcast:  map[int]bool{1: true},
			want:       []int{1, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &orderRecorder{}
			s := New()
			for id, p := range tc.priorities {
				if tc.broadcast[id] {
					s.AddBroadcastHandlerWithPriority(rec.handler(id), p)
					continue
				}
				s.AddHandlerWithPriority(rec.handler(id), p, syscall.SIGUSR1)
			}

			s.runHandlers(syscall.SIGUSR1)

			require.Equal(t, tc.want, rec.snapshot())
		})
	}
}

// TestRunHandlers_LargeSetUsesPriorityOrder registers more handlers than
// LargeHandlerCountThreshold, which routes dispatch through the priority
// queue, and mixes predefined and custom priorities with some predefined
// levels left empty.
func TestRunHandlers_LargeSetUsesPriorityOrder(t *testing.T) {
	t.Parallel()

	priorities := []Priority{PriorityLow, 200, PriorityHighest, 10, -5}
	rec := &orderRecorder{}
	s := New()

	const n = LargeHandlerCountThreshold + 10
	want := make([]int, 0, n)
	byPriority := map[Priority][]int{}
	for id := range n {
		p := priorities[id%len(priorities)]
		byPriority[p] = append(byPriority[p], id)
		if id%2 == 0 {
			s.AddHandlerWithPriority(rec.handler(id), p, syscall.SIGUSR1)
		} else {
			s.AddBroadcastHandlerWithPriority(rec.handler(id), p)
		}
	}

	// Within a priority level broadcast handlers come first, each group in
	// registration order.
	for _, p := range []Priority{200, PriorityHighest, PriorityLow, 10, -5} {
		ids := byPriority[p]
		for _, id := range ids {
			if id%2 == 1 {
				want = append(want, id)
			}
		}
		for _, id := range ids {
			if id%2 == 0 {
				want = append(want, id)
			}
		}
	}

	s.runHandlers(syscall.SIGUSR1)

	require.Equal(t, want, rec.snapshot())
}

// TestRunHandlers_SparsePrioritiesFallBackToInsertionSort uses a handler
// count between the insertion-sort threshold and the priority-queue
// threshold with a priority range too wide for counting sort.
func TestRunHandlers_SparsePrioritiesFallBackToInsertionSort(t *testing.T) {
	t.Parallel()

	rec := &orderRecorder{}
	s := New()

	const n = insertionSortThreshold + 8
	var high, low []int
	for id := range n {
		p := Priority(1)
		if id%3 == 0 {
			p = 1_000_000
			high = append(high, id)
		} else {
			low = append(low, id)
		}
		s.AddHandlerWithPriority(rec.handler(id), p, syscall.SIGUSR1)
	}

	s.runHandlers(syscall.SIGUSR1)

	require.Equal(t, append(high, low...), rec.snapshot())
}

func TestRunHandlers_SequentialStopsAtTimedHandlerAfterShutdown(t *testing.T) {
	t.Parallel()

	rec := &orderRecorder{}
	s := New()
	s.globalCancel()

	s.broadcastHandlers = []handlerEntry{
		{contextHandler: rec.handler(1), timeout: 0, priority: PriorityHighest},
		{contextHandler: rec.handler(2), timeout: time.Second, priority: PriorityHigh},
		{contextHandler: rec.handler(3), timeout: 0, priority: PriorityNormal},
	}

	s.runHandlers(syscall.SIGUSR1)

	// Only handlers with a timeout check for shutdown; the first one that
	// does ends the sequence, so later handlers do not run either.
	require.Equal(t, []int{1}, rec.snapshot())
}

func TestRunHandlers_ParallelRunsEveryHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		poolSize int
	}{
		{name: "worker pool has room", poolSize: 10},
		{name: "full worker pool runs handlers synchronously", poolSize: 0},
		{name: "worker pool smaller than handler count", poolSize: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				rec := &orderRecorder{}
				s := New(WithExecutionMode(ParallelMode), WithWorkerPoolSize(tc.poolSize))
				for id := range 4 {
					s.AddHandler(rec.handler(id), syscall.SIGUSR1)
				}

				s.runHandlers(syscall.SIGUSR1)

				require.ElementsMatch(t, []int{0, 1, 2, 3}, rec.snapshot())
				require.Empty(t, s.workerPool, "every worker slot must be released")
			})
		})
	}
}

func TestRunHandlers_ParallelSkipsHandlersAfterShutdown(t *testing.T) {
	t.Parallel()

	for _, poolSize := range []int{0, 10} {
		t.Run(fmt.Sprintf("pool size %d", poolSize), func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				rec := &orderRecorder{}
				s := New(WithExecutionMode(ParallelMode), WithWorkerPoolSize(poolSize))
				s.AddHandler(rec.handler(1), syscall.SIGUSR1)
				s.AddBroadcastHandler(rec.handler(2))
				s.globalCancel()

				s.runHandlers(syscall.SIGUSR1)

				require.Empty(t, rec.snapshot())
			})
		})
	}
}

// TestRunHandlersParallel_ShutdownGracePeriodUsesClock checks that a parallel
// dispatch whose handler ignores cancellation returns after the grace period
// measured by the injected clock instead of waiting for the handler.
func TestRunHandlersParallel_ShutdownGracePeriodUsesClock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		fake := clock.NewFake(start)
		s := New(WithExecutionMode(ParallelMode), WithClock(fake))

		started := make(chan struct{})
		release := make(chan struct{})
		var finished sync.WaitGroup
		finished.Add(1)
		entry := handlerEntry{
			contextHandler: func(context.Context, os.Signal) error {
				defer finished.Done()
				close(started)
				<-release
				return nil
			},
		}

		returned := make(chan struct{})
		go func() {
			defer close(returned)
			s.runHandlersParallelOptimized([]handlerEntry{entry}, syscall.SIGUSR1)
		}()

		<-started
		s.globalCancel()
		<-returned

		require.Equal(t, start.Add(ResponsiveTimeoutDuration), fake.Now())

		close(release)
		finished.Wait()
	})
}

func TestRunHandlersParallel_EmptyIsNoop(t *testing.T) {
	t.Parallel()

	fake := clock.NewFake(time.Time{})
	s := New(WithExecutionMode(ParallelMode), WithClock(fake))
	s.globalCancel()

	s.runHandlersParallelOptimized(nil, syscall.SIGUSR1)

	require.True(t, fake.Now().IsZero(), "an empty dispatch must not wait for the grace period")
}

func TestExecuteHandlerEntry_ReportsFailures(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	tests := []struct {
		name    string
		timeout time.Duration
		handler ContextHandler
		check   func(t *testing.T, err error)
	}{
		{
			name:    "returned error without timeout",
			handler: func(context.Context, os.Signal) error { return errBoom },
			check:   func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		{
			name:    "returned error with timeout",
			timeout: time.Second,
			handler: func(context.Context, os.Signal) error { return errBoom },
			check:   func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		{
			name:    "panic without timeout",
			handler: func(context.Context, os.Signal) error { panic("kaboom") },
			check: func(t *testing.T, err error) {
				pe, ok := errors.AsType[*PanicError](err)
				require.True(t, ok, "want *PanicError, got %T", err)
				require.Equal(t, "kaboom", pe.Panic)
				require.Equal(t, syscall.SIGUSR1, pe.Signal)
			},
		},
		{
			name:    "panic with timeout",
			timeout: time.Second,
			handler: func(context.Context, os.Signal) error { panic(errBoom) },
			check: func(t *testing.T, err error) {
				pe, ok := errors.AsType[*PanicError](err)
				require.True(t, ok, "want *PanicError, got %T", err)
				require.Equal(t, errBoom, pe.Panic)
				require.Equal(t, syscall.SIGUSR1, pe.Signal)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				rec := &errorRecorder{}
				s := New(WithErrorHandler(rec.handle))
				s.executeHandlerEntryOptimized(handlerEntry{contextHandler: tc.handler, timeout: tc.timeout}, syscall.SIGUSR1)
				s.wg.Wait()

				errs, sigs := rec.snapshot()
				require.Len(t, errs, 1)
				require.Equal(t, []os.Signal{syscall.SIGUSR1}, sigs)
				tc.check(t, errs[0])
			})
		})
	}
}

func TestExecuteHandlerEntry_SuccessReportsNothing(t *testing.T) {
	t.Parallel()

	for _, timeout := range []time.Duration{0, time.Second} {
		rec := &errorRecorder{}
		s := New(WithErrorHandler(rec.handle))
		s.executeHandlerEntryOptimized(handlerEntry{
			contextHandler: func(context.Context, os.Signal) error { return nil },
			timeout:        timeout,
		}, syscall.SIGUSR1)
		s.wg.Wait()

		errs, _ := rec.snapshot()
		require.Empty(t, errs, "timeout %v", timeout)
	}
}

func TestExecuteHandlerEntry_TimeoutReportsTimeoutError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		rec := &errorRecorder{}
		s := New(WithErrorHandler(rec.handle))
		release := make(chan struct{})
		const timeout = 3 * time.Second

		// The handler ignores its context so the dispatcher deterministically
		// observes the deadline before any handler result.
		s.executeHandlerEntryOptimized(handlerEntry{
			contextHandler: func(context.Context, os.Signal) error {
				<-release
				return nil
			},
			timeout: timeout,
		}, syscall.SIGHUP)

		errs, sigs := rec.snapshot()
		require.Len(t, errs, 1)
		require.Equal(t, []os.Signal{syscall.SIGHUP}, sigs)
		require.True(t, IsTimeout(errs[0]))
		te, ok := errors.AsType[*TimeoutError](errs[0])
		require.True(t, ok)
		require.Equal(t, timeout, te.Timeout)
		require.Equal(t, syscall.SIGHUP, te.Signal)

		// The abandoned handler goroutine stays tracked by s.wg.
		close(release)
		s.wg.Wait()
	})
}

func TestExecuteHandlerEntry_ShutdownIsNotReportedAsTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		rec := &errorRecorder{}
		s := New(WithErrorHandler(rec.handle))
		started := make(chan struct{})
		release := make(chan struct{})

		returned := make(chan struct{})
		go func() {
			defer close(returned)
			s.executeHandlerEntryOptimized(handlerEntry{
				contextHandler: func(context.Context, os.Signal) error {
					close(started)
					<-release
					return nil
				},
				timeout: time.Hour,
			}, syscall.SIGTERM)
		}()

		<-started
		s.globalCancel()
		<-returned

		errs, _ := rec.snapshot()
		require.Empty(t, errs)

		close(release)
		s.wg.Wait()
	})
}

func TestSafeCallErrorHandler(t *testing.T) {
	t.Parallel()

	t.Run("nil error handler is ignored", func(t *testing.T) {
		t.Parallel()
		s := New()
		require.NotPanics(t, func() { s.safeCallErrorHandler(syscall.SIGUSR1, errors.New("x")) })
	})

	t.Run("panicking error handler is contained", func(t *testing.T) {
		t.Parallel()
		calls := 0
		s := New(WithErrorHandler(func(os.Signal, error) {
			calls++
			panic("error handler failed")
		}))
		require.NotPanics(t, func() { s.safeCallErrorHandler(syscall.SIGUSR1, errors.New("x")) })
		require.Equal(t, 1, calls)
	})

	t.Run("handler panic with panicking error handler", func(t *testing.T) {
		t.Parallel()
		s := New(WithErrorHandler(func(os.Signal, error) { panic("again") }))
		entry := handlerEntry{contextHandler: func(context.Context, os.Signal) error { panic("first") }}
		require.NotPanics(t, func() { s.executeHandlerEntryOptimized(entry, syscall.SIGUSR1) })
	})
}

func TestAddHandlers_NilHandlerIsIgnored(t *testing.T) {
	t.Parallel()

	s := New()
	require.Same(t, s, s.AddHandler(nil, syscall.SIGUSR1))
	require.Same(t, s, s.AddBroadcastHandler(nil))
	require.Same(t, s, s.AddHandlerWithPriority(nil, PriorityHighest, syscall.SIGUSR1))

	require.Empty(t, s.handlers)
	require.Empty(t, s.broadcastHandlers)
}

func TestAddHandler_RegistersForEverySignal(t *testing.T) {
	t.Parallel()

	rec := &orderRecorder{}
	s := New()
	s.AddHandler(rec.handler(1), syscall.SIGUSR1, syscall.SIGUSR2)

	s.runHandlers(syscall.SIGUSR1)
	s.runHandlers(syscall.SIGUSR2)
	s.runHandlers(syscall.SIGHUP)

	require.Equal(t, []int{1, 1}, rec.snapshot())
}

// TestSignalsListener_FullWorkerPoolDropsSignal feeds the listener directly
// through its channel: with no worker slots the signal is dropped and the
// error handler is told.
func TestSignalsListener_FullWorkerPoolDropsSignal(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		rec := &errorRecorder{}
		ran := false
		s := New(WithWorkerPoolSize(0), WithErrorHandler(rec.handle))
		s.AddHandler(func(context.Context, os.Signal) error {
			ran = true
			return nil
		}, syscall.SIGUSR1)

		s.wg.Go(s.signalsListener)
		s.signalChannel <- syscall.SIGUSR1
		synctest.Wait()
		s.stop <- struct{}{}
		s.wg.Wait()

		errs, sigs := rec.snapshot()
		require.Len(t, errs, 1)
		require.Error(t, errs[0])
		require.Equal(t, []os.Signal{syscall.SIGUSR1}, sigs)
		require.False(t, ran)
	})
}

func TestSignalsListener_DispatchesToWorker(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		rec := &orderRecorder{}
		s := New()
		s.AddHandler(rec.handler(7), syscall.SIGUSR1)

		s.wg.Go(s.signalsListener)
		s.signalChannel <- syscall.SIGUSR1
		synctest.Wait()
		s.stop <- struct{}{}
		s.wg.Wait()

		require.Equal(t, []int{7}, rec.snapshot())
		require.Empty(t, s.workerPool)
	})
}
