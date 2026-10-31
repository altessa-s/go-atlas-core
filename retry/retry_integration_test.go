//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that run the retry loop on the system clock with
// real timers. Lower bounds are exact (a timer never fires early);
// upper bounds are generous so a loaded machine cannot fail them.

package retry_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/retry"
)

// slack bounds how late a real timer or a cancellation may be observed.
const slack = 2 * time.Second

var errTransient = errors.New("transient")

// attemptLog records when each attempt started.
type attemptLog struct {
	mu    sync.Mutex
	times []time.Time
}

func (l *attemptLog) fn(err error) func(context.Context) error {
	return func(context.Context) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.times = append(l.times, time.Now())
		return err
	}
}

func (l *attemptLog) snapshot() []time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]time.Time(nil), l.times...)
}

func TestDo_ExponentialBackoffWaitsRealDelays(t *testing.T) {
	t.Parallel()

	const (
		base    = 5 * time.Millisecond
		retries = 4
	)
	next := retry.Exponential(base, retry.WithExponentialFactor(2))
	var log attemptLog
	var delays []time.Duration

	start := time.Now()
	err := retry.Do(t.Context(), log.fn(errTransient),
		retry.WithMaxAttempts(retries),
		retry.WithShouldRetry(func(error) bool { return true }),
		retry.WithNextDelay(next),
		retry.WithOnRetry(func(_ int, _ error, d time.Duration) { delays = append(delays, d) }),
	)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, errTransient)
	times := log.snapshot()
	require.Len(t, times, retries+1)
	require.Equal(t, []time.Duration{base, 2 * base, 4 * base, 8 * base}, delays)
	var total time.Duration
	for i, d := range delays {
		gap := times[i+1].Sub(times[i])
		require.GreaterOrEqual(t, gap, d, "attempt %d started before its backoff elapsed", i+1)
		total += d
	}
	require.GreaterOrEqual(t, elapsed, total)
	require.Less(t, elapsed, total+slack)
}

func TestDo_CancelInterruptsLongWait(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	var log attemptLog
	time.AfterFunc(20*time.Millisecond, cancel)

	start := time.Now()
	err := retry.Do(ctx, log.fn(errTransient),
		retry.WithMaxAttempts(-1),
		retry.WithNextDelay(func(int, error) time.Duration { return time.Hour }),
	)

	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, errTransient)
	require.Len(t, log.snapshot(), 1)
	require.Less(t, time.Since(start), slack)
}

func TestDo_JoinLastErrorOnRealDeadline(t *testing.T) {
	t.Parallel()

	const timeout = 50 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	var log attemptLog

	start := time.Now()
	err := retry.Do(ctx, log.fn(errTransient),
		retry.WithMaxAttempts(-1),
		retry.WithNextDelay(func(int, error) time.Duration { return 10 * time.Millisecond }),
		retry.WithJoinLastError(),
	)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorIs(t, err, errTransient)
	require.GreaterOrEqual(t, elapsed, timeout)
	require.Less(t, elapsed, timeout+slack)
	require.GreaterOrEqual(t, len(log.snapshot()), 2, "a 10ms backoff under a 50ms deadline must retry")
}

func TestDo_StopBeforeDeadlineSkipsDoomedWait(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var log attemptLog

	start := time.Now()
	err := retry.Do(ctx, log.fn(errTransient),
		retry.WithMaxAttempts(-1),
		retry.WithNextDelay(func(int, error) time.Duration { return time.Hour }),
		retry.WithStopBeforeDeadline(),
	)

	// The hour-long wait would outlive the minute deadline, so Do
	// returns the attempt's error at once instead of sleeping into it.
	require.ErrorIs(t, err, errTransient)
	require.NoError(t, ctx.Err())
	require.Len(t, log.snapshot(), 1)
	require.Less(t, time.Since(start), slack)
}

func TestDo_MaxElapsedTimeStopsOnRealClock(t *testing.T) {
	t.Parallel()

	const budget = 60 * time.Millisecond
	var log attemptLog

	start := time.Now()
	err := retry.Do(t.Context(), log.fn(errTransient),
		retry.WithMaxAttempts(-1),
		retry.WithMaxElapsedTime(budget),
		retry.WithNextDelay(func(int, error) time.Duration { return 10 * time.Millisecond }),
	)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, errTransient)
	require.GreaterOrEqual(t, len(log.snapshot()), 2)
	require.Less(t, elapsed, budget+slack)
}

func TestDo_ConcurrentLoopsShareNothing(t *testing.T) {
	t.Parallel()

	// Many loops on real timers at once: each must make exactly its own
	// number of attempts, which a timer shared between calls would break.
	const loops = 32
	var wg sync.WaitGroup
	errs := make([]error, loops)
	counts := make([]int, loops)
	for i := range loops {
		wg.Go(func() {
			errs[i] = retry.Do(t.Context(), func(context.Context) error {
				counts[i]++
				if counts[i] <= i%4 {
					return errTransient
				}
				return nil
			},
				retry.WithMaxAttempts(5),
				retry.WithNextDelay(retry.Exponential(time.Millisecond, retry.WithExponentialJitter(0.5))),
			)
		})
	}
	wg.Wait()
	for i := range loops {
		require.NoError(t, errs[i], "loop %d", i)
		require.Equal(t, i%4+1, counts[i], "loop %d", i)
	}
}
