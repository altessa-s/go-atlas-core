// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/altessa-s/go-atlas-core/retry"
)

func BenchmarkDo(b *testing.B) {
	ctx := b.Context()
	failErr := errors.New("fail")

	// Benchmark overhead with immediate success (default: single attempt, no retries).
	b.Run("Success", func(b *testing.B) {
		for b.Loop() {
			_ = retry.Do(ctx, func(ctx context.Context) error {
				return nil
			})
		}
	})

	// Benchmark overhead with immediate retries.
	b.Run("Retries", func(b *testing.B) {
		opts := []retry.Option{
			retry.WithMaxAttempts(10),
			retry.WithNextDelay(func(int, error) time.Duration { return 0 }), // No delay
		}
		for b.Loop() {
			_ = retry.Do(ctx, func(ctx context.Context) error {
				return failErr
			}, opts...)
		}
	})

	b.Run("ExponentialCalc", func(b *testing.B) {
		exp := retry.Exponential(time.Second)
		i := 0
		for b.Loop() {
			exp(i%10, nil)
			i++
		}
	})

	// Prebuilt-policy variants of the Success/Retries cases: options are
	// materialized once instead of on every call.
	b.Run("PolicySuccess", func(b *testing.B) {
		p := retry.NewPolicy()
		b.ReportAllocs()
		for b.Loop() {
			_ = p.Do(ctx, func(ctx context.Context) error {
				return nil
			})
		}
	})

	b.Run("PolicyRetries", func(b *testing.B) {
		p := retry.NewPolicy(
			retry.WithMaxAttempts(10),
			retry.WithNextDelay(func(int, error) time.Duration { return 0 }),
		)
		b.ReportAllocs()
		for b.Loop() {
			_ = p.Do(ctx, func(ctx context.Context) error {
				return failErr
			})
		}
	})
}

// BenchmarkDo_JoinLastError measures a retry whose wait the context cuts
// short, returning the last attempt's error joined with ctx.Err().
func BenchmarkDo_JoinLastError(b *testing.B) {
	failErr := errors.New("fail")
	ctx, cancel := context.WithCancel(b.Context())
	cancel()
	policy := retry.NewPolicy(
		retry.WithAttemptOnDoneContext(),
		retry.WithJoinLastError(),
		retry.WithMaxAttempts(3),
		retry.WithNextDelay(func(int, error) time.Duration { return time.Second }),
	)
	fn := func(context.Context) error { return failErr }
	for b.Loop() {
		_ = policy.Do(ctx, fn)
	}
}

// BenchmarkExponential measures the delay computation with the options a
// production policy sets: a cap and random jitter, sequentially and from
// many goroutines sharing one NextDelayFunc.
func BenchmarkExponential(b *testing.B) {
	next := retry.Exponential(100*time.Millisecond,
		retry.WithExponentialFactor(2),
		retry.WithExponentialMaxDelay(30*time.Second),
		retry.WithExponentialJitter(0.2),
	)
	b.Run("JitterCapped", func(b *testing.B) {
		b.ReportAllocs()
		var sink time.Duration
		i := 0
		for b.Loop() {
			sink += next(i%16, nil)
			i++
		}
		if sink <= 0 {
			b.Fatal("non-positive delays")
		}
	})
	b.Run("JitterCappedParallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if next(i%16, nil) <= 0 {
					b.Error("non-positive delay")
					return
				}
				i++
			}
		})
	})
}

// BenchmarkDo_RealTimerWait measures a retry loop that really waits on the
// system clock between attempts, which exercises the reused timer.
func BenchmarkDo_RealTimerWait(b *testing.B) {
	failErr := errors.New("fail")
	ctx := b.Context()
	policy := retry.NewPolicy(
		retry.WithMaxAttempts(3),
		retry.WithNextDelay(func(int, error) time.Duration { return time.Microsecond }),
	)
	b.ReportAllocs()
	for b.Loop() {
		calls := 0
		err := policy.Do(ctx, func(context.Context) error {
			calls++
			if calls < 4 {
				return failErr
			}
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
