// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package signals_test

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/signals"
)

// startBlockedHandler starts s, delivers SIGUSR2 to the test process and
// waits until handler has begun running.
func startBlockedHandler(tb testing.TB, s *signals.Signal, started <-chan struct{}) {
	tb.Helper()
	s.Start()

	p, err := os.FindProcess(os.Getpid())
	require.NoError(tb, err)
	require.NoError(tb, p.Signal(syscall.SIGUSR2))

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		require.FailNow(tb, "handler did not start")
	}
}

// Shutdown documents that a handler ignoring its context keeps running after
// Shutdown returns on timeout, and that Wait blocks until it exits. Once the
// handler is released everything the Signal started exits: nothing leaks.
//
// Serial, like the other tests here: signal.Notify registrations are
// process-wide.
func TestSignal_ShutdownTimeout_HandlerOutlivesShutdown(t *testing.T) {
	testhelpers.VerifyNoLeaks(t)

	started := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool

	s := signals.New(
		signals.WithSignals(syscall.SIGUSR2),
		signals.WithShutdownHooks(func(context.Context) error { return nil }),
	)
	s.AddHandler(func(context.Context, os.Signal) error {
		close(started)
		<-release // ignores ctx on purpose
		finished.Store(true)
		return nil
	}, syscall.SIGUSR2)
	startBlockedHandler(t, s, started)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, s.Shutdown(ctx), context.DeadlineExceeded)
	require.False(t, finished.Load(), "the handler is still running after Shutdown timed out")

	waited := make(chan struct{})
	go func() {
		s.Wait()
		close(waited)
	}()
	require.Never(t, func() bool {
		select {
		case <-waited:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond, "Wait returned while the handler is still running")

	// The blocked handler is not a leak: release is reachable from here.
	close(release)
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Wait did not return after the handler was released")
	}
	require.True(t, finished.Load())
}

// When the handler that outlives a timed-out Shutdown is blocked on something
// nothing else can reach, it can never finish, and the goroutineleak profile
// reports it together with the waiter goroutine Shutdown started to watch the
// handler WaitGroup: that waiter is blocked on s.wg, which only the leaked
// handler could release. This is the documented Shutdown timeout behavior
// ("their goroutines are not reclaimed"), not a defect in Shutdown: the leak
// is caused by the handler ignoring its context.
//
// The test leaks its goroutines for the rest of the test binary on purpose.
// They carry this test's pprof label, so no other leak check attributes them.
//
// Serial, like the other tests here: signal.Notify registrations are
// process-wide.
func TestSignal_ShutdownTimeout_UnreachableHandlerLeaksWaiter(t *testing.T) {
	tracker := testhelpers.TrackGoroutines()
	started := make(chan struct{})

	s := signals.New(
		signals.WithSignals(syscall.SIGUSR2),
		signals.WithShutdownHooks(func(context.Context) error { return nil }),
	)
	s.AddHandler(func(context.Context, os.Signal) error {
		close(started)
		<-make(chan struct{}) // ignores ctx and blocks on a channel nobody can reach
		return nil
	}, syscall.SIGUSR2)
	startBlockedHandler(t, s, started)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, s.Shutdown(ctx), context.DeadlineExceeded)
	// s is not used after this point, so its WaitGroup stays reachable only
	// from the leaked goroutines.

	var leaked []string
	require.Eventually(t, func() bool {
		var err error
		leaked, err = tracker.Leaked()
		return err == nil && hasStack(leaked, "TestSignal_ShutdownTimeout_UnreachableHandlerLeaksWaiter.func") &&
			hasStack(leaked, "signals.(*Signal).Shutdown.func")
	}, 5*time.Second, 10*time.Millisecond, "expected the handler and the Shutdown waiter to be reported as leaked")
	t.Logf("goroutineleak profile for this test:\n%s", strings.Join(leaked, "\n\n"))
}

// hasStack reports whether any leaked stack record mentions frame.
func hasStack(records []string, frame string) bool {
	for _, r := range records {
		if strings.Contains(r, frame) {
			return true
		}
	}
	return false
}
