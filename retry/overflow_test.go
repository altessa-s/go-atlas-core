// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package retry_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/retry"
)

func TestExponential_JitterOverflow(t *testing.T) {
	t.Parallel()
	const base = time.Duration(math.MaxInt64 / 4 * 3)
	for _, limit := range []time.Duration{0, math.MaxInt64 - 1} {
		fn := retry.Exponential(base, retry.WithExponentialMaxDelay(limit), retry.WithExponentialJitter(1))
		for range 100 {
			delay := fn(0, nil)
			require.GreaterOrEqual(t, delay, base)
			if limit > 0 {
				require.LessOrEqual(t, delay, limit)
			}
		}
	}
}

func TestDo_ElapsedBudgetOverflow(t *testing.T) {
	t.Parallel()
	failure := errors.New("failed attempt")
	slept := false
	err := retry.Do(t.Context(), func(context.Context) error { return failure },
		retry.WithMaxAttempts(1),
		retry.WithMaxElapsedTime(time.Second),
		retry.WithNextDelay(func(int, error) time.Duration { return math.MaxInt64 }),
		retry.WithSleep(func(context.Context, time.Duration) error { slept = true; return nil }),
	)
	require.ErrorIs(t, err, failure)
	require.False(t, slept, "a delay beyond the elapsed budget must not be slept")
}
