// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/retry"
	"github.com/altessa-s/go-atlas-core/time/clock"
)

// fixedDelay retries after d whatever the attempt.
func fixedDelay(d time.Duration) retry.Option {
	return retry.WithNextDelay(func(int, error) time.Duration { return d })
}

// sleepClock is a clock.Clock that reads a fake time and waits with sleep.
type sleepClock struct {
	*clock.Fake
	sleep func(context.Context, time.Duration) error
}

func (c sleepClock) Sleep(ctx context.Context, d time.Duration) error { return c.sleep(ctx, d) }

// withSleep injects a clock whose Sleep runs sleep instead of waiting.
func withSleep(sleep func(context.Context, time.Duration) error) retry.Option {
	return retry.WithClock(sleepClock{Fake: clock.NewFake(time.Time{}), sleep: sleep})
}

func TestWithClock(t *testing.T) {
	t.Parallel()

	t.Run("drives the backoff", func(t *testing.T) {
		t.Parallel()

		var slept []time.Duration
		calls := 0
		err := retry.Do(t.Context(), func(context.Context) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		},
			retry.WithMaxAttempts(5),
			retry.WithNextDelay(func(attempt int, _ error) time.Duration { return time.Duration(attempt+1) * time.Hour }),
			withSleep(func(_ context.Context, d time.Duration) error {
				slept = append(slept, d)
				return nil
			}),
		)
		require.NoError(t, err)
		require.Equal(t, 3, calls)
		require.Equal(t, []time.Duration{time.Hour, 2 * time.Hour}, slept, "no real sleep, delays as computed")
	})

	t.Run("fake clock sleeps without waiting", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		fake := clock.NewFake(start)
		calls := 0
		err := retry.Do(t.Context(), func(context.Context) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		}, retry.WithMaxAttempts(5), fixedDelay(time.Hour), retry.WithClock(fake))
		require.NoError(t, err)
		require.Equal(t, 3, calls)
		require.Equal(t, start.Add(2*time.Hour), fake.Now(), "both waits ran on the fake clock")
	})

	t.Run("nil keeps the system clock", func(t *testing.T) {
		t.Parallel()

		calls := 0
		err := retry.Do(t.Context(), func(context.Context) error {
			calls++
			if calls < 2 {
				return errors.New("transient")
			}
			return nil
		}, retry.WithMaxAttempts(1), fixedDelay(time.Nanosecond), retry.WithClock(nil))
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("its error ends Do", func(t *testing.T) {
		t.Parallel()

		errSleep := errors.New("sleep interrupted")
		calls := 0
		err := retry.Do(t.Context(), func(context.Context) error {
			calls++
			return errors.New("transient")
		},
			retry.WithMaxAttempts(5), fixedDelay(time.Hour),
			withSleep(func(context.Context, time.Duration) error { return errSleep }),
		)
		require.ErrorIs(t, err, errSleep)
		require.Equal(t, 1, calls)
	})
}

func TestWithStopBeforeDeadline(t *testing.T) {
	t.Parallel()

	errAttempt := errors.New("provider busy")

	t.Run("returns the last error instead of sleeping into the deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		calls, retries := 0, 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			return errAttempt
		},
			retry.WithMaxAttempts(5), fixedDelay(time.Hour), retry.WithStopBeforeDeadline(),
			retry.WithOnRetry(func(int, error, time.Duration) { retries++ }),
		)
		require.ErrorIs(t, err, errAttempt)
		require.NotErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, calls)
		require.Zero(t, retries, "no retry is announced")
	})

	t.Run("retries whose delay ends before the deadline run", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		calls := 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			if calls < 3 {
				return errAttempt
			}
			return nil
		},
			retry.WithMaxAttempts(5), fixedDelay(time.Millisecond), retry.WithStopBeforeDeadline(),
		)
		require.NoError(t, err)
		require.Equal(t, 3, calls)
	})

	t.Run("no deadline: no effect", func(t *testing.T) {
		t.Parallel()

		calls := 0
		err := retry.Do(context.WithoutCancel(t.Context()), func(context.Context) error {
			calls++
			return errAttempt
		},
			retry.WithMaxAttempts(2), fixedDelay(time.Hour), retry.WithStopBeforeDeadline(),
			withSleep(func(context.Context, time.Duration) error { return nil }),
		)
		require.ErrorIs(t, err, errAttempt)
		require.Equal(t, 3, calls)
	})

	t.Run("fake clock measures the time left", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			remaining time.Duration
			wantCalls int
		}{
			{name: "stops once the delay would outlast the deadline", remaining: 10 * time.Second, wantCalls: 4},
			{name: "a delay equal to the time left stops", remaining: 9 * time.Second, wantCalls: 3},
			{name: "deadline sooner than the first delay", remaining: 2 * time.Second, wantCalls: 1},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fake := clock.NewFake(time.Now())
				ctx, cancel := context.WithDeadline(t.Context(), fake.Now().Add(tc.remaining))
				defer cancel()
				calls := 0
				err := retry.Do(ctx, func(context.Context) error {
					calls++
					return errAttempt
				},
					retry.WithMaxAttempts(-1), fixedDelay(3*time.Second), retry.WithStopBeforeDeadline(),
					retry.WithClock(fake),
				)
				require.ErrorIs(t, err, errAttempt)
				require.NotErrorIs(t, err, context.DeadlineExceeded)
				require.Equal(t, tc.wantCalls, calls)
			})
		}
	})

	t.Run("without the option the deadline ends the sleep", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		err := retry.Do(ctx, func(context.Context) error { return errAttempt },
			retry.WithMaxAttempts(5), fixedDelay(time.Hour))
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestWithMaxElapsedTime(t *testing.T) {
	t.Parallel()

	errAttempt := errors.New("provider busy")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		attempt   time.Duration // fake time each attempt takes
		wantCalls int
		wantNow   time.Time
	}{
		{name: "stops before a delay that exceeds the budget", wantCalls: 4, wantNow: start.Add(9 * time.Second)},
		{name: "time spent in attempts counts", attempt: 2 * time.Second, wantCalls: 3, wantNow: start.Add(12 * time.Second)},
		{name: "an attempt that uses up the budget stops", attempt: 10 * time.Second, wantCalls: 1, wantNow: start.Add(10 * time.Second)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := clock.NewFake(start)
			calls := 0
			err := retry.Do(t.Context(), func(context.Context) error {
				calls++
				fake.Advance(tc.attempt)
				return errAttempt
			},
				retry.WithMaxAttempts(-1), fixedDelay(3*time.Second),
				retry.WithMaxElapsedTime(10*time.Second), retry.WithClock(fake),
			)
			require.ErrorIs(t, err, errAttempt)
			require.Equal(t, tc.wantCalls, calls)
			require.Equal(t, tc.wantNow, fake.Now())
		})
	}
}

// retryAfterError is a failure that asks for a minimum pause.
type retryAfterError struct{ after time.Duration }

func (retryAfterError) Error() string { return "busy" }

// TestNextDelayRaisedFromError shows the Retry-After pattern: the delay
// callback receives the failed attempt's error and raises the backoff to the
// minimum the error carries.
func TestNextDelayRaisedFromError(t *testing.T) {
	t.Parallel()

	backoff := retry.Exponential(time.Millisecond, retry.WithExponentialFactor(1))
	var slept []time.Duration
	calls := 0
	err := retry.Do(t.Context(), func(context.Context) error {
		calls++
		if calls == 1 {
			return retryAfterError{after: 3 * time.Second}
		}
		return nil
	},
		retry.WithMaxAttempts(1),
		retry.WithNextDelay(func(attempt int, err error) time.Duration {
			d := backoff(attempt, err)
			if ra, ok := errors.AsType[retryAfterError](err); ok {
				d = max(d, ra.after)
			}
			return d
		}),
		withSleep(func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		}),
	)
	require.NoError(t, err)
	require.Equal(t, []time.Duration{3 * time.Second}, slept)
}
