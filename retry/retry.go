// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry

import (
	"cmp"
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"time"

	"github.com/altessa-s/go-atlas-core/time/clock"

	corecontext "github.com/altessa-s/go-atlas-core/context"
)

// Do calls fn repeatedly according to the policy supplied via opts until
// one of the following occurs:
//   - fn returns nil (success) — Do returns nil.
//   - ctx is canceled — Do returns ctx.Err() (joined with the last attempt's
//     error under [WithJoinLastError]). A ctx that has already ended returns
//     before the first attempt unless [WithAttemptOnDoneContext] is set.
//   - maxAttempts is exhausted — Do returns the last error from fn.
//   - maxElapsedTime is exceeded — Do returns the last error from fn.
//   - The shouldRetry filter returns false — Do returns the error immediately.
//   - The nextDelay callback is nil or returns ≤ 0 — Do returns the error
//     immediately.
//   - With [WithStopBeforeDeadline], the next delay would reach the context
//     deadline — Do returns the last error from fn rather than ctx.Err().
//   - The [WithClock] clock's Sleep returns an error — Do returns that error
//     (joined with the last attempt's error under [WithJoinLastError]).
//
// The delay before a retry comes from the [WithNextDelay] callback, which
// receives the failed attempt's error: a policy that must wait at least as
// long as the error asks (an HTTP Retry-After, a quota reset) raises the
// backoff there, e.g. max(backoff(attempt, err), retryAfterOf(err)).
//
// Every time read and wait goes through the [WithClock] clock (the system
// clock by default). A nil ctx is silently replaced with
// [context.Background]. Without any options the call behaves as a single
// attempt (no retries): pass [WithMaxAttempts] and [WithNextDelay] to enable
// retrying.
func Do(ctx context.Context, fn func(context.Context) error, opts ...Option) error {
	return doWithOptions(ctx, fn, newOptions(opts...))
}

// Policy is a retry configuration materialized once via [NewPolicy], letting
// hot call paths (e.g. a per-request HTTP attempt) reuse it across calls
// without re-processing options on every invocation. A Policy is immutable
// after construction and safe for concurrent use.
type Policy struct {
	cfg *options
}

// NewPolicy materializes opts into a reusable [Policy]. With no options the
// policy behaves like [Do] without options: a single attempt, no retries.
func NewPolicy(opts ...Option) *Policy {
	return &Policy{cfg: newOptions(opts...)}
}

// Do runs fn under the pre-built policy with the same semantics as the
// package-level [Do].
func (p *Policy) Do(ctx context.Context, fn func(context.Context) error) error {
	return doWithOptions(ctx, fn, p.cfg)
}

//nolint:contextcheck // nil ctx is replaced with Background for convenience (see Do docs).
func doWithOptions(ctx context.Context, fn func(context.Context) error, cfg *options) error {
	ctx = corecontext.OrBackground(ctx)

	start := cfg.clock.Now()

	var (
		timer   *time.Timer
		lastErr error
	)
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for attempt := 0; cfg.maxAttempts < 0 || attempt <= cfg.maxAttempts; attempt++ {
		if ctx.Err() != nil && (attempt > 0 || !cfg.attemptOnDoneContext) {
			return cfg.contextEnded(ctx, lastErr)
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if cfg.shouldRetry != nil && !cfg.shouldRetry(err) {
			return err
		}

		delay, ok := cfg.retryDelay(ctx, start, attempt, err)
		if !ok {
			return err
		}

		if cfg.onRetry != nil {
			cfg.onRetry(attempt, err, delay)
		}

		if werr := cfg.waitRetry(ctx, &timer, delay, lastErr); werr != nil {
			return werr
		}
	}

	return lastErr
}

// waitRetry waits delay before the next attempt, with the reusable *timer
// (allocated on first use, to avoid time.After allocations in a loop) on the
// system clock, or with the configured clock's Sleep otherwise. It returns nil
// once the wait is over, or the error that ends the retry loop when the
// context or the sleep cut it short.
func (cfg *options) waitRetry(ctx context.Context, timer **time.Timer, delay time.Duration, lastErr error) error {
	if !isRealClock(cfg.clock) {
		serr := cfg.clock.Sleep(ctx, delay)
		if serr == nil {
			return nil
		}
		// A sleep ended by the context reports it as the timer does,
		// so a clock.Clock sleep keeps context.Cause.
		if cerr := ctx.Err(); cerr != nil && errors.Is(serr, cerr) {
			return cfg.contextEnded(ctx, lastErr)
		}
		return cfg.interrupted(lastErr, serr)
	}

	if *timer == nil {
		*timer = time.NewTimer(delay)
	} else {
		(*timer).Reset(delay)
	}

	select {
	case <-(*timer).C:
		return nil
	case <-ctx.Done():
		return cfg.contextEnded(ctx, lastErr)
	}
}

// realClockType is the dynamic type of the system clock returned by clock.Real.
var realClockType = reflect.TypeOf(clock.Real())

// isRealClock reports whether c is the system clock, whose Sleep waiting can
// be replaced with the reusable timer of waitRetry. It compares dynamic types
// rather than interface values, so it does not depend on clock.Real returning
// one comparable value: an interface comparison would panic if the system
// clock type ever became non-comparable, and would miss distinct instances
// if it became a pointer. It does not allocate.
func isRealClock(c clock.Clock) bool {
	return reflect.TypeOf(c) == realClockType
}

// retryDelay returns the wait before the retry that follows the failed
// attempt, and false when there is no retry: the last permitted attempt
// failed, no delay is configured or it is not positive, the delay would
// exceed the elapsed-time budget, or the context deadline ends first.
func (cfg *options) retryDelay(ctx context.Context, start time.Time, attempt int, err error) (time.Duration, bool) {
	// The last permitted attempt failed: return now rather than compute a
	// delay, notify onRetry and sleep for a retry that never comes.
	if cfg.maxAttempts >= 0 && attempt >= cfg.maxAttempts {
		return 0, false
	}

	if cfg.nextDelay == nil {
		return 0, false
	}

	delay := cfg.nextDelay(attempt, err)
	if delay <= 0 {
		return 0, false
	}

	if cfg.maxElapsedTime > 0 {
		elapsed := cfg.clock.Now().Sub(start)
		if elapsed >= cfg.maxElapsedTime {
			return 0, false
		}
		if delay > cfg.maxElapsedTime-elapsed {
			return 0, false
		}
	}

	if cfg.stopBeforeDeadline {
		if deadline, ok := ctx.Deadline(); ok && deadline.Sub(cfg.clock.Now()) <= delay {
			return 0, false
		}
	}
	return delay, true
}

// contextEnded is the error Do returns when ctx ended before an attempt:
// ctx.Err(), or with [WithJoinLastError] and a failed attempt before it,
// lastErr joined with ctx.Err() and, when it differs, context.Cause(ctx).
func (cfg *options) contextEnded(ctx context.Context, lastErr error) error {
	err := ctx.Err()
	if !cfg.joinLastError || lastErr == nil {
		return err
	}
	if cause := context.Cause(ctx); cause != err {
		return errors.Join(lastErr, err, cause)
	}
	return errors.Join(lastErr, err)
}

// interrupted is the error Do returns when the wait before a retry failed
// with err: err, or with [WithJoinLastError] lastErr joined with err.
func (cfg *options) interrupted(lastErr, err error) error {
	if !cfg.joinLastError || lastErr == nil {
		return err
	}
	return errors.Join(lastErr, err)
}

// Exponential returns a [NextDelayFunc] that implements exponential
// backoff with optional jitter, suitable for [WithNextDelay]:
//
//	delay(attempt) = min(maxDelay, baseDelay * factor^attempt + jitter)
//
// A zero or negative baseDelay yields 0 for every attempt, which stops [Do]
// from retrying. [WithExponentialMaxDelay] caps the delay (unbounded by
// default), [WithExponentialFactor] sets the multiplier (default
// [DefaultExponentialFactor]) and [WithExponentialJitter] adds random jitter
// (deterministic by default). The returned function is safe for concurrent
// use.
func Exponential(baseDelay time.Duration, opts ...ExponentialOption) NextDelayFunc {
	o := newExponentialOptions(opts...)
	maxDelay := o.maxDelay
	factor := cmp.Or(o.factor, DefaultExponentialFactor)
	jitter := min(o.jitter, 1.0)

	return func(attempt int, _ error) time.Duration {
		if baseDelay <= 0 {
			return 0
		}
		if attempt < 0 {
			attempt = 0
		}

		// math.Pow returns +Inf for large attempt counts (e.g. factor=2,
		// attempt=64). The resulting product overflows the float64→int64
		// conversion and yields math.MinInt64 on current Go runtimes —
		// which the "d <= 0" guard below would silently turn into a
		// zero-delay hot loop, defeating maxDelay entirely. Detect the
		// overflow up front and clamp to maxDelay (or the maximum
		// representable duration when MaxDelay is uncapped).
		multiplier := math.Pow(factor, float64(attempt))
		product := float64(baseDelay) * multiplier
		if math.IsInf(multiplier, 1) || product >= float64(math.MaxInt64) {
			if maxDelay > 0 {
				return maxDelay
			}
			return time.Duration(math.MaxInt64)
		}

		d := time.Duration(product)
		if d <= 0 {
			return 0
		}
		if maxDelay > 0 && d > maxDelay {
			d = maxDelay
		}

		if jitter > 0 {
			limit := time.Duration(math.MaxInt64)
			if maxDelay > 0 {
				limit = maxDelay
			}
			extra := float64(d) * jitter * rand.Float64() // #nosec G404 -- non-cryptographic jitter for retry backoff timing
			if extra >= float64(limit-d) {
				return limit
			}
			d += time.Duration(extra)
		}

		return d
	}
}
