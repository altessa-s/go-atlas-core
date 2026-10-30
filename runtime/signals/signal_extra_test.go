// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package signals_test

import (
	"context"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/signals"
)

// The tests in this file are serial: signal.Notify registrations and the
// signals sent to the test process are process-wide. They cannot run in a
// synctest bubble either: the signal package delivers to the subscribed
// channel from the runtime's signal goroutine, which is outside any bubble,
// and operating on a bubbled channel from outside the bubble panics. They
// therefore poll with require.Eventually on real time instead of sleeping a
// fixed duration.

// handlerWait bounds how long a test waits for a signal to reach its handler.
const handlerWait = 2 * time.Second

// pollInterval is how often a test re-checks whether a handler has run.
const pollInterval = 5 * time.Millisecond

func TestSignal_AddHandler_Start_Stop(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)
	var called atomic.Bool

	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithHandlerTimeout(2*time.Second),
		signals.WithShutdownTimeout(2*time.Second),
	)

	s.AddHandler(func(ctx context.Context, sig os.Signal) error {
		called.Store(true)
		return nil
	}, syscall.SIGUSR1)

	s.Start()

	// Send signal to self
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)

	require.Eventually(t, called.Load, handlerWait, pollInterval, "handler was not called")
	require.NoError(t, s.Stop())
}

func TestSignal_AddHandlerWithPriority(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)
	// order is written by the handlers and read only after Stop, which waits
	// for them; ran tells the test when both have run.
	var order []int
	var ran atomic.Int32
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR2),
		signals.WithExecutionMode(signals.SequentialMode),
		signals.WithHandlerTimeout(2*time.Second),
	)

	s.AddHandlerWithPriority(func(_ context.Context, _ os.Signal) error {
		order = append(order, 2)
		ran.Add(1)
		return nil
	}, signals.PriorityLow, syscall.SIGUSR2)

	s.AddHandlerWithPriority(func(_ context.Context, _ os.Signal) error {
		order = append(order, 1)
		ran.Add(1)
		return nil
	}, signals.PriorityHigh, syscall.SIGUSR2)

	s.Start()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR2)

	require.Eventually(t, func() bool { return ran.Load() == 2 }, handlerWait, pollInterval, "handlers did not run")
	require.NoError(t, s.Stop())

	require.Len(t, order, 2)
	// Higher priority should run first
	require.Equal(t, 1, order[0], "expected high priority first, got order %v", order)
}

func TestSignal_BroadcastHandler(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)
	var called atomic.Bool
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithHandlerTimeout(2*time.Second),
	)

	s.AddBroadcastHandler(func(_ context.Context, _ os.Signal) error {
		called.Store(true)
		return nil
	})

	s.Start()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)

	require.Eventually(t, called.Load, handlerWait, pollInterval, "broadcast handler was not called")
	require.NoError(t, s.Stop())
}

func TestSignal_Shutdown(t *testing.T) {
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithShutdownTimeout(1*time.Second),
	)
	s.Start()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	require.NoError(t, s.Shutdown(ctx))
}

// A rejected nil-context Shutdown must leave the handler running, so a later
// valid Shutdown still stops the listener and Wait returns.
//
// Serial, like the other tests here: signal.Notify registrations are
// process-wide.
func TestSignal_ShutdownNilContext_KeepsHandlerStoppable(t *testing.T) {
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithShutdownTimeout(1*time.Second),
	)
	s.Start()

	//nolint:staticcheck // SA1012: a nil context is the input under test.
	require.Error(t, s.Shutdown(nil))

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))

	waited := make(chan struct{})
	go func() {
		s.Wait()
		close(waited)
	}()
	require.Eventually(t, func() bool {
		select {
		case <-waited:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond, "Wait did not return: the listener is still running")
}

func TestSignal_ParallelMode(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithExecutionMode(signals.ParallelMode),
		signals.WithWorkerPoolSize(5),
		signals.WithHandlerTimeout(2*time.Second),
	)

	var count atomic.Int32
	for range 3 {
		s.AddHandler(func(_ context.Context, _ os.Signal) error {
			count.Add(1)
			return nil
		}, syscall.SIGUSR1)
	}

	s.Start()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)

	require.Eventually(t, func() bool { return count.Load() == 3 }, handlerWait, pollInterval,
		"expected 3 handlers called in parallel, got %d", count.Load())
	require.NoError(t, s.Stop())
}

func TestSignal_ErrorHandler(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)
	var errCalled atomic.Bool
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithHandlerTimeout(50*time.Millisecond),
		signals.WithErrorHandler(func(sig os.Signal, err error) {
			errCalled.Store(true)
		}),
	)

	// The handler outlives its 50ms timeout and only returns once its context
	// expires; whichever of the two the dispatcher observes first, the error
	// handler is called (a TimeoutError or the context error).
	s.AddHandler(func(ctx context.Context, _ os.Signal) error {
		<-ctx.Done()
		return ctx.Err()
	}, syscall.SIGUSR1)

	s.Start()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGUSR1)

	require.Eventually(t, errCalled.Load, handlerWait, pollInterval, "error handler was not called")
	require.NoError(t, s.Stop())
}
