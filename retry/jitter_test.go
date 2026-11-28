// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExponentialJitterMode(t *testing.T) {
	t.Parallel()

	const base = time.Second
	for _, tc := range []struct {
		name    string
		mode    JitterMode
		jitter  float64
		attempt int
		limit   time.Duration
		low     time.Duration // delay at u = 0
		middle  time.Duration // delay at u = 0.5
		high    time.Duration // upper bound
	}{
		{"zero additive", JitterAdditive, 0, 1, 0, 2 * base, 2 * base, 2 * base},
		{"zero symmetric", JitterSymmetric, 0, 1, 0, 2 * base, 2 * base, 2 * base},
		{"negative max delay is unbounded", JitterAdditive, 0, 1, -base, 2 * base, 2 * base, 2 * base},
		{"additive", JitterAdditive, 0.5, 0, 0, base, 5 * base / 4, 3 * base / 2},
		{"unknown", JitterMode(99), 0.5, 0, 0, base, 5 * base / 4, 3 * base / 2},
		{"symmetric", JitterSymmetric, 0.5, 0, 0, base / 2, base, 3 * base / 2},
		{"capped before jitter", JitterSymmetric, 0.5, 2, 2 * base, base, 2 * base, 2 * base},
		{"capped after jitter", JitterSymmetric, 1, 0, 3 * base / 2, 0, base, 3 * base / 2},
		{"product overflow", JitterSymmetric, 1, 1024, base, 0, base, base},
		{"duration overflow", JitterSymmetric, 1, 1024, 0, 0, math.MaxInt64, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var u float64

			opts := newExponentialOptions(WithExponentialFactor(2), WithExponentialJitter(tc.jitter),
				WithExponentialMaxDelay(tc.limit), WithExponentialJitterMode(tc.mode))
			next := exponential(base, opts, func() float64 { return u })
			require.Equal(t, tc.low, next(tc.attempt, nil))

			u = 0.5

			require.Equal(t, tc.middle, next(tc.attempt, nil))

			for i := range 256 {
				u = float64(i) / 256
				d := next(tc.attempt, nil)
				require.GreaterOrEqual(t, d, tc.low)
				require.LessOrEqual(t, d, tc.high)
			}
		})
	}
}

func TestExponentialFullJitter(t *testing.T) {
	t.Parallel()

	const base = time.Second
	for _, tc := range []struct {
		name    string
		jitter  float64
		attempt int
		limit   time.Duration
		low     time.Duration // delay at u = 0, at least 1ns so Do keeps retrying
		middle  time.Duration // delay at u = 0.5
		ceiling time.Duration // the capped delay, never reached unless jitter is zero
	}{
		{"zero jitter", 0, 1, 0, 2 * base, 2 * base, 2 * base},
		{"full", 1, 1, 0, 1, base, 2 * base},
		{"partial", 0.5, 0, 0, base / 2, 3 * base / 4, base},
		{"capped", 1, 3, 2 * base, 1, base, 2 * base},
		{"product overflow", 1, 1024, base, 1, base / 2, base},
		{"duration overflow", 0.5, 1024, 0, 1 << 62, 3 << 61, math.MaxInt64}, // float64(math.MaxInt64) is 2^63
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var u float64

			next := Exponential(base, WithExponentialFactor(2), WithExponentialJitter(tc.jitter), WithExponentialMaxDelay(tc.limit),
				WithExponentialJitterMode(JitterFull), WithExponentialJitterSource(func() float64 { return u }))
			require.Equal(t, tc.low, next(tc.attempt, nil))

			u = 0.5

			require.Equal(t, tc.middle, next(tc.attempt, nil))

			for i := range 256 {
				u = float64(i) / 256
				d := next(tc.attempt, nil)
				require.GreaterOrEqual(t, d, tc.low)

				if tc.jitter > 0 {
					require.Less(t, d, tc.ceiling)
				} else {
					require.Equal(t, tc.ceiling, d)
				}
			}
		})
	}
}

// TestExponentialFullJitterBounds keeps a full-jitter delay positive and within the capped delay at the extremes of the
// sample, where rounding could otherwise reach the cap or overflow the duration.
func TestExponentialFullJitterBounds(t *testing.T) {
	t.Parallel()

	below := math.Nextafter(1, 0)

	for _, tc := range []struct {
		name    string
		jitter  float64
		attempt int
		limit   time.Duration
	}{
		{"unbounded overflow", 0.25, 1024, 0},
		{"unbounded full", 1, 1024, 0},
		{"capped", 0.25, 3, time.Minute},
		{"nanoseconds", 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, u := range []float64{0, math.SmallestNonzeroFloat64, 1e-12, 0.5, below} {
				next := Exponential(time.Nanosecond, WithExponentialFactor(2), WithExponentialJitter(tc.jitter),
					WithExponentialMaxDelay(tc.limit), WithExponentialJitterMode(JitterFull),
					WithExponentialJitterSource(func() float64 { return u }))
				d := next(tc.attempt, nil)
				require.Positive(t, d, "u = %v", u)

				limit := tc.limit
				if limit == 0 {
					limit = math.MaxInt64
				}

				require.LessOrEqual(t, d, limit, "u = %v", u)
			}
		})
	}
}

// TestExponentialJitterSourceClamped keeps samples of a custom source inside [0, 1), so a faulty source can neither
// shorten an additive delay nor push it past the cap.
func TestExponentialJitterSourceClamped(t *testing.T) {
	t.Parallel()

	const base = time.Second
	for _, tc := range []struct {
		name   string
		sample float64
		want   time.Duration
	}{
		{"negative", -3, base},
		{"NaN", math.NaN(), base},
		{"negative infinity", math.Inf(-1), base},
		{"one", 1, 2*base - 1},
		{"above one", 7, 2*base - 1},
		{"positive infinity", math.Inf(1), 2*base - 1},
		{"inside", 0.25, 5 * base / 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			next := Exponential(base, WithExponentialJitter(1), WithExponentialJitterSource(func() float64 { return tc.sample }))
			require.Equal(t, tc.want, next(0, nil))
		})
	}

	require.InDelta(t, 0, unitSample(-0.0), 0)
	require.Less(t, unitSample(1), 1.0)
}

// TestExponentialFullJitterKeepsRetrying runs Do with full jitter whose source draws 0: the shortest delay is positive,
// so every attempt runs.
func TestExponentialFullJitterKeepsRetrying(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := Do(t.Context(), func(context.Context) error {
		attempts++

		return errFlaky
	},
		WithMaxAttempts(3),
		WithShouldRetry(func(error) bool { return true }),
		WithNextDelay(Exponential(time.Microsecond, WithExponentialJitter(1), WithExponentialJitterMode(JitterFull),
			WithExponentialJitterSource(func() float64 { return 0 }))),
	)
	require.ErrorIs(t, err, errFlaky)
	require.Equal(t, 4, attempts)
}

var errFlaky = errors.New("flaky")
