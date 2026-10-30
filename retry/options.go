// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry

//go:generate optgen generate

import (
	"time"

	"github.com/altessa-s/go-atlas-core/time/clock"
)

// ShouldRetryFunc decides whether [Do] should retry after err. Returning
// false causes [Do] to return err immediately without further attempts.
type ShouldRetryFunc func(err error) bool

// NextDelayFunc returns the delay before the next attempt. It receives
// the zero-based attempt number and the error from the most recent call.
// A non-positive result stops retrying and [Do] returns the last error.
// Use [Exponential] to build a standard exponential-backoff delay.
type NextDelayFunc func(attempt int, err error) time.Duration

// OnRetryFunc is an optional callback invoked after each failed attempt,
// before the delay sleep. It receives the attempt number, the error, and
// the computed delay that will be applied.
type OnRetryFunc func(attempt int, err error, nextDelay time.Duration)

// options holds the retry policy used by [Do]. Configure it through the
// generated [Option] values (see the With* constructors below).
//
// Attempt numbering starts at 0: maxAttempts=0 means "try once" (no
// retries), maxAttempts=3 means attempts 0..3 (4 total calls to fn),
// maxAttempts=-1 disables the attempt cap entirely.
type options struct {
	// maxAttempts is the maximum attempt index (inclusive). A value of 0
	// means the function is called exactly once; -1 disables the cap.
	// The field is deliberately unconstrained so that the zero value is a
	// legal configuration (single-shot with no retries).
	maxAttempts int `opt:"-"`

	// maxElapsedTime caps the total time, as read from clock, spent across
	// all attempts. A zero or negative value disables the limit.
	maxElapsedTime time.Duration `optval:"nonpositive"`

	// shouldRetry, when non-nil, filters retryable errors. A nil value
	// retries every error that reaches the retry loop.
	shouldRetry ShouldRetryFunc //

	// nextDelay computes the backoff before each retry. A nil value or a
	// non-positive return stops retrying.
	nextDelay NextDelayFunc //

	// onRetry is an optional callback invoked after each failed attempt.
	onRetry OnRetryFunc //

	// clock is the single time source of the retry loop: it starts and
	// checks the maxElapsedTime budget, measures the time left before the
	// context deadline for stopBeforeDeadline, and waits between attempts.
	// Its Sleep error ends Do as is, except an error matching ctx.Err() while
	// ctx has ended, which Do reports as a context end (context.Cause
	// included). Pass a clock.Fake to drive the loop in tests; a nil value
	// is ignored.
	clock clock.Clock `optgen:"default=clock.Real(),notnil"`

	// stopBeforeDeadline, set by the generated WithStopBeforeDeadline, skips
	// a retry whose backoff would reach the context deadline: Do returns the
	// last attempt's error instead of sleeping into the deadline and
	// returning ctx.Err().
	stopBeforeDeadline bool

	// joinLastError, set by the generated WithJoinLastError, keeps the
	// failure that caused a retry when the context ends before that retry
	// starts (during the wait, or right after it) or the clock's Sleep fails:
	// Do returns errors.Join(lastErr, ctx.Err()[, context.Cause(ctx)]) or
	// errors.Join(lastErr, sleepErr) instead of the context or sleep error
	// alone, so errors.Is matches both the attempt's failure and the
	// interruption.
	joinLastError bool

	// attemptOnDoneContext, set by the generated WithAttemptOnDoneContext,
	// makes the first attempt even when ctx has already ended, so fn decides
	// how a done context fails (and the failure is reported like any other
	// attempt's) instead of Do returning ctx.Err() without calling fn. Later
	// attempts still stop on a done context.
	attemptOnDoneContext bool
}

// WithMaxAttempts sets the maximum attempt index (inclusive). A value of
// 0 means fn is called exactly once, a positive N means up to N retries
// (N+1 total calls), and -1 disables the cap. The zero value of
// [options] already corresponds to "single attempt", so callers only
// need this option when they want retries.
func WithMaxAttempts(n int) Option {
	return func(o *options) {
		o.maxAttempts = n
	}
}
