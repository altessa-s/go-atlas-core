// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry

import (
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
