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

var errAttempt = errors.New("attempt failed")

// cancelDuringWait returns options whose OnRetry cancels the context, so the
// wait before the second attempt ends with the context.
func cancelDuringWait(cancel context.CancelFunc, extra ...retry.Option) []retry.Option {
	return append([]retry.Option{
		retry.WithMaxAttempts(3),
		retry.WithNextDelay(func(int, error) time.Duration { return time.Hour }),
		retry.WithOnRetry(func(int, error, time.Duration) { cancel() }),
	}, extra...)
}

func TestDo_JoinLastError(t *testing.T) {
	t.Parallel()

	t.Run("WaitInterruptedByCancel", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		calls := 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			return errAttempt
		}, cancelDuringWait(cancel, retry.WithJoinLastError())...)

		require.Equal(t, 1, calls)
		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("DefaultKeepsContextErrorOnly", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := retry.Do(ctx, func(context.Context) error { return errAttempt }, cancelDuringWait(cancel)...)

		require.Equal(t, context.Canceled, err)
	})

	t.Run("CauseIsJoined", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("shutting down")
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)

		err := retry.Do(ctx, func(context.Context) error { return errAttempt },
			cancelDuringWait(func() { cancel(cause) }, retry.WithJoinLastError())...)

		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, cause)
	})

	t.Run("SleepEndedByContextKeepsCause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("shutting down")
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)

		// A clock.Clock sleep returns ctx.Err() when the context ends: Do
		// reports it as the timer does, cause included.
		err := retry.Do(ctx, func(context.Context) error { return errAttempt },
			cancelDuringWait(func() { cancel(cause) }, retry.WithJoinLastError(),
				retry.WithSleep(clock.Real().Sleep))...)

		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, cause)
	})

	t.Run("DeadlineDuringWait", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()

		err := retry.Do(ctx, func(context.Context) error { return errAttempt },
			retry.WithMaxAttempts(3),
			retry.WithNextDelay(func(int, error) time.Duration { return time.Hour }),
			retry.WithJoinLastError(),
		)

		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("ContextEndsAfterWait", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		// The sleep succeeds, but the context ended meanwhile: the check
		// before the next attempt reports it.
		calls := 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			return errAttempt
		},
			retry.WithMaxAttempts(3),
			retry.WithNextDelay(func(int, error) time.Duration { return time.Millisecond }),
			retry.WithSleep(func(context.Context, time.Duration) error { cancel(); return nil }),
			retry.WithJoinLastError(),
		)

		require.Equal(t, 1, calls)
		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("SleepFailure", func(t *testing.T) {
		t.Parallel()
		errSleep := errors.New("clock stopped")

		err := retry.Do(t.Context(), func(context.Context) error { return errAttempt },
			retry.WithMaxAttempts(3),
			retry.WithNextDelay(func(int, error) time.Duration { return time.Second }),
			retry.WithSleep(func(context.Context, time.Duration) error { return errSleep }),
			retry.WithJoinLastError(),
		)

		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, errSleep)
	})

	t.Run("ExhaustedReturnsLastErrorOnly", func(t *testing.T) {
		t.Parallel()

		err := retry.Do(t.Context(), func(context.Context) error { return errAttempt },
			retry.WithMaxAttempts(2),
			retry.WithNextDelay(func(int, error) time.Duration { return time.Nanosecond }),
			retry.WithJoinLastError(),
		)

		require.Equal(t, errAttempt, err)
	})

	t.Run("DoneBeforeFirstAttempt", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		// No attempt failed, so there is nothing to join.
		err := retry.Do(ctx, func(context.Context) error { return errAttempt }, retry.WithJoinLastError())

		require.Equal(t, context.Canceled, err)
	})
}

func TestDo_AttemptOnDoneContext(t *testing.T) {
	t.Parallel()

	t.Run("FirstAttemptRuns", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		calls := 0
		err := retry.Do(ctx, func(ctx context.Context) error {
			calls++
			return ctx.Err()
		}, retry.WithAttemptOnDoneContext())

		require.Equal(t, 1, calls)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("FirstAttemptMaySucceed", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := retry.Do(ctx, func(context.Context) error { return nil }, retry.WithAttemptOnDoneContext())

		require.NoError(t, err)
	})

	t.Run("NoRetryOnDoneContext", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		calls := 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			return errAttempt
		},
			retry.WithAttemptOnDoneContext(),
			retry.WithJoinLastError(),
			retry.WithMaxAttempts(5),
			retry.WithNextDelay(func(int, error) time.Duration { return time.Nanosecond }),
		)

		require.Equal(t, 1, calls)
		require.ErrorIs(t, err, errAttempt)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("DefaultSkipsAttempt", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		calls := 0
		err := retry.Do(ctx, func(context.Context) error {
			calls++
			return nil
		})

		require.Zero(t, calls)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Policy", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		policy := retry.NewPolicy(retry.WithAttemptOnDoneContext())
		calls := 0
		err := policy.Do(ctx, func(context.Context) error {
			calls++
			return errAttempt
		})

		require.Equal(t, 1, calls)
		require.Equal(t, errAttempt, err)
	})
}
