// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package signals_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/signals"
)

// Serial, like the other tests here: signal.Notify registrations are
// process-wide.
func TestSignal_Shutdown_RunsInjectedShutdownHooks(t *testing.T) {
	errHooks := errors.New("hooks failed")
	var calls atomic.Int32

	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithShutdownHooks(func(context.Context) error {
			calls.Add(1)
			return errHooks
		}),
	)
	s.Start()

	err := s.Shutdown(t.Context())
	require.ErrorIs(t, err, errHooks)
	require.Equal(t, int32(1), calls.Load())

	require.NoError(t, s.Shutdown(t.Context()), "second Shutdown is a no-op")
	require.Equal(t, int32(1), calls.Load())
}

// Serial, like the other tests here: signal.Notify registrations are
// process-wide.
func TestSignal_ShutdownTimeout_RunsInjectedShutdownHooks(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	s := signals.New(
		signals.WithSignals(syscall.SIGUSR2),
		signals.WithShutdownHooks(func(context.Context) error {
			calls.Add(1)
			return nil
		}),
	)
	// The handler ignores its context, so Shutdown can only time out.
	s.AddHandler(func(context.Context, os.Signal) error {
		close(started)
		<-release
		return nil
	}, syscall.SIGUSR2)
	s.Start()

	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, p.Signal(syscall.SIGUSR2))

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "handler did not start")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, s.Shutdown(ctx), context.DeadlineExceeded)
	require.Equal(t, int32(1), calls.Load())
}
