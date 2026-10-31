// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight_test

import (
	"context"
	"runtime"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/singleflight"
)

func TestAdmissionAndCancellation(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	g := singleflight.New[string, string](singleflight.WithMaxConcurrent(1))
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		_, err := g.Do(ctx, "a", func(work context.Context) (string, error) {
			close(entered)
			<-release
			return "a", work.Err()
		})
		done <- err
	}()
	<-entered
	t.Cleanup(func() { close(release) })
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	for i := range 1000 {
		_, err := g.Do(t.Context(), strconv.Itoa(i), func(context.Context) (string, error) { panic("overloaded work started") })
		require.ErrorIs(t, err, singleflight.ErrOverloaded)
	}
	wait, stop := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer stop()
	_, err := g.Do(wait, "a", func(context.Context) (string, error) { panic("duplicate computation") })
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDistinctKeysAndNilResult(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	g := singleflight.New[any, any]()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := g.Do(t.Context(), 1, func(context.Context) (any, error) {
			close(entered)
			<-release
			return "integer", nil
		})
		done <- err
	}()
	<-entered
	t.Cleanup(func() { close(release); <-done })
	v, err := g.Do(t.Context(), "1", func(context.Context) (any, error) { return nil, nil })
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestTimeoutAndAbnormalExit(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	g := singleflight.New[string, int](singleflight.WithMaxConcurrent(1), singleflight.WithTimeout(time.Millisecond))
	_, err := g.Do(t.Context(), "timeout", func(ctx context.Context) (int, error) { <-ctx.Done(); return 0, ctx.Err() })
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.PanicsWithValue(t, "boom", func() {
		_, _ = g.Do(t.Context(), "panic", func(context.Context) (int, error) { panic("boom") })
	})
	_, err = g.Do(t.Context(), "exit", func(context.Context) (int, error) { runtime.Goexit(); return 0, nil })
	require.ErrorIs(t, err, singleflight.ErrAborted)
	v, err := g.Do(t.Context(), "next", func(context.Context) (int, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, 42, v)
}

// A non-positive WithTimeout is ignored: the shared computation keeps the
// DefaultTimeout deadline. The bubble's fake clock makes the deadline exact.
func TestNonPositiveTimeoutKeepsDefault(t *testing.T) {
	t.Parallel()
	for _, timeout := range []time.Duration{0, -time.Second} {
		synctest.Test(t, func(t *testing.T) {
			g := singleflight.New[string, time.Duration](singleflight.WithTimeout(timeout))
			start := time.Now()
			got, err := g.Do(t.Context(), "key", func(work context.Context) (time.Duration, error) {
				deadline, ok := work.Deadline()
				require.True(t, ok, "the computation context must carry a deadline")
				return deadline.Sub(start), nil
			})
			require.NoError(t, err)
			require.Equal(t, singleflight.DefaultTimeout, got)
		})
	}
}

func TestCanceledCallDoesNotStartWork(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	g := singleflight.New[string, int]()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := g.Do(ctx, "key", func(context.Context) (int, error) { panic("canceled work started") })
	require.ErrorIs(t, err, context.Canceled)
}

// Work every caller abandoned keeps its capacity until the group's timeout
// ends it, then exits and releases the capacity; its goroutine does not leak.
// The bubble's fake clock makes the timeout instant and exact.
func TestAbandonedWorkExitsOnGroupTimeout(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	const timeout = 20 * time.Millisecond
	synctest.Test(t, func(t *testing.T) {
		g := singleflight.New[string, int](singleflight.WithMaxConcurrent(1), singleflight.WithTimeout(timeout))
		entered := make(chan struct{})
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			<-entered
			cancel()
		}()
		_, err := g.Do(ctx, "abandoned", func(work context.Context) (int, error) {
			close(entered)
			<-work.Done()
			return 0, work.Err()
		})
		require.ErrorIs(t, err, context.Canceled)

		next := func(context.Context) (int, error) { return 1, nil }
		_, err = g.Do(t.Context(), "next", next)
		require.ErrorIs(t, err, singleflight.ErrOverloaded, "the abandoned work still holds the capacity")

		synctest.Sleep(timeout)
		v, err := g.Do(t.Context(), "next", next)
		require.NoError(t, err)
		require.Equal(t, 1, v)
	})
}
