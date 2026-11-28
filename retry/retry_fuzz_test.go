// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/retry"
)

func FuzzExponential(f *testing.F) {
	f.Add(int64(time.Second), int64(time.Minute), 2.0, 5, 0.0)  // deterministic
	f.Add(int64(time.Second), int64(time.Minute), 2.0, 5, 0.25) // with jitter
	f.Add(int64(time.Second), int64(time.Minute), 1.5, 0, 1.0)  // max jitter
	f.Add(int64(time.Second), int64(time.Minute), 2.0, 10, 5.0) // jitter > 1 (clamped)

	f.Fuzz(func(t *testing.T, base int64, maxDelay int64, factor float64, attempt int, jitter float64) {
		fn := retry.Exponential(time.Duration(base),
			retry.WithExponentialMaxDelay(time.Duration(maxDelay)),
			retry.WithExponentialFactor(factor),
			retry.WithExponentialJitter(jitter),
		)
		got := fn(attempt, nil)

		require.GreaterOrEqual(t, got, time.Duration(0), "Exponential returned negative duration")

		if maxDelay > 0 && time.Duration(maxDelay) > 0 {
			require.LessOrEqual(t, got, time.Duration(maxDelay), "Exponential returned duration > MaxDelay")
		}
	})
}

// FuzzExponentialJitterModes checks every jitter mode against a fixed sample: the delay stays within [0, maxDelay],
// full jitter never exceeds the deterministic delay and stays positive whenever it is.
func FuzzExponentialJitterModes(f *testing.F) {
	f.Add(int64(time.Second), int64(time.Minute), 2.0, 5, 1.0, 0, 0.5)
	f.Add(int64(time.Second), int64(0), 2.0, 1024, 0.5, 2, 0.999999)
	f.Add(int64(time.Nanosecond), int64(0), 1.5, 3, 1.0, 2, 0.0)
	f.Add(int64(time.Millisecond), int64(time.Second), 3.0, 40, 0.75, 1, -2.0)

	f.Fuzz(func(t *testing.T, base, maxDelay int64, factor float64, attempt int, jitter float64, mode int, sample float64) {
		opts := []retry.ExponentialOption{
			retry.WithExponentialMaxDelay(time.Duration(maxDelay)),
			retry.WithExponentialFactor(factor),
			retry.WithExponentialJitterMode(retry.JitterMode(mode)),
		}
		deterministic := retry.Exponential(time.Duration(base), opts...)(attempt, nil)
		got := retry.Exponential(time.Duration(base), append(opts, retry.WithExponentialJitter(jitter),
			retry.WithExponentialJitterSource(func() float64 { return sample }))...)(attempt, nil)

		require.GreaterOrEqual(t, got, time.Duration(0))

		if maxDelay > 0 {
			require.LessOrEqual(t, got, time.Duration(maxDelay))
		}

		if retry.JitterMode(mode) == retry.JitterFull {
			require.LessOrEqual(t, got, deterministic)

			if deterministic > 0 {
				require.Positive(t, got)
			}
		}
	})
}
