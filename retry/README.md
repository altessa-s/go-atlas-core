# retry

```go
import "github.com/altessa-s/go-atlas-core/retry"
```

Package `retry` provides a small, stdlib-only retry loop with context cancellation, configurable delay policies, and timer reuse — zero `time.After`
allocations per attempt, so it fits high-throughput retry loops in hot paths.

## Quick start

```go
err := retry.Do(ctx, func(ctx context.Context) error {
    return callExternalAPI(ctx)
},
    retry.WithMaxAttempts(3), // attempts 0..3 (4 total calls)
    retry.WithNextDelay(retry.Exponential(
        500*time.Millisecond,
        retry.WithExponentialMaxDelay(10*time.Second),
        retry.WithExponentialFactor(1.5),
        retry.WithExponentialJitter(0.2),
    )),
    retry.WithShouldRetry(func(err error) bool {
        return !errors.Is(err, ErrNotFound) // don't retry on not-found
    }),
    retry.WithOnRetry(func(attempt int, err error, delay time.Duration) {
        logger.Warn("retrying", slog.Int("attempt", attempt), slog.Any("error", err))
    }),
)
```

## Stop conditions

`Do` returns when any of the following occurs:

- `fn` returns nil — success.
- `ctx` is canceled — `ctx.Err()` (joined with the last attempt's error under `WithJoinLastError`); a context that has already ended returns before
  the first attempt unless `WithAttemptOnDoneContext` is set.
- `WithMaxAttempts` is exhausted — the last error from `fn`.
- `WithMaxElapsedTime` is exceeded — the last error from `fn`.
- the `WithShouldRetry` filter returns `false` — the error, immediately.
- the `WithNextDelay` callback is nil or returns `<= 0` — the error, immediately.
- with `WithStopBeforeDeadline`, the next delay would reach the context deadline — the last error from `fn`, not `ctx.Err()`.
- the `WithClock` clock's `Sleep` returns an error — that error (joined with the last attempt's error under `WithJoinLastError`).

## Options

| Option               | Description                                              |
|----------------------|----------------------------------------------------------|
| `WithMaxAttempts`    | Max attempt index (0 = try once, -1 = unlimited)         |
| `WithMaxElapsedTime` | Cap on the clock time spent across all attempts          |
| `WithShouldRetry`    | Predicate; `false` stops retrying immediately            |
| `WithNextDelay`      | Delay function `(attempt, err) -> Duration`; `<=0` stops |
| `WithOnRetry`        | Optional callback before each retry's sleep              |
| `WithClock`          | Time source for the elapsed budget, the deadline check and the wait between attempts (default `clock.Real()`, a `clock.Fake` in tests); a `Sleep` error ends `Do`, a context end is reported as by the timer; `nil` is ignored |
| `WithStopBeforeDeadline` | Skip a retry whose delay would reach the context deadline; `Do` returns the last attempt's error, not `ctx.Err()` |
| `WithJoinLastError`  | A context end or sleep failure before a retry returns `errors.Join(lastErr, cause)`, not the cause alone |
| `WithAttemptOnDoneContext` | Make the first attempt even when the context has already ended, so `fn` reports how a done context fails |

With no options `Do` calls `fn` exactly once: retrying needs both a non-zero `WithMaxAttempts` and a `WithNextDelay` policy.

## Deadlines and minimum delays

By default a retry whose backoff outlasts the context sleeps until the deadline and `Do` returns `ctx.Err()`. `WithStopBeforeDeadline()` checks the
remaining time first: when the next delay would reach the deadline no retry is started (and `WithOnRetry` is not called) and `Do` returns the error of
the last attempt, so callers keep the real failure class instead of a generic deadline error.

When the context ends during the wait before a retry, `Do` returns `ctx.Err()` by default and the failure that caused the retry is lost.
`WithJoinLastError()` returns `errors.Join(lastErr, ctx.Err())` instead (plus `context.Cause(ctx)` when it differs), and joins a clock `Sleep` error the
same way, so `errors.Is` matches both. A context that has already ended returns `ctx.Err()` before the first attempt; `WithAttemptOnDoneContext()` makes
that attempt anyway, so the caller's `fn` classifies and reports the failure like any other (later attempts still stop on a done context):

```go
err := retry.Do(ctx, call,
    retry.WithMaxAttempts(2),
    retry.WithNextDelay(backoff),
    retry.WithJoinLastError(),        // errors.Is(err, errUpstream) && errors.Is(err, context.Canceled)
    retry.WithAttemptOnDoneContext(), // a done ctx still yields call's own error
)
```

The `WithNextDelay` callback receives the failed attempt's error, so a minimum taken from the error — an HTTP `Retry-After`, a quota reset — is applied
there:

```go
retry.WithNextDelay(func(attempt int, err error) time.Duration {
    return max(backoff(attempt, err), retryAfterOf(err))
})
```

## Prebuilt policies

`Do(ctx, fn, opts...)` materializes its options on every call. Hot call paths (e.g. a per-request HTTP attempt) should build a `Policy` once and reuse
it — the options are processed a single time and each `Policy.Do` call skips option processing (a call that retries on the default clock still
allocates one reusable timer):

```go
var policy = retry.NewPolicy(retry.WithMaxAttempts(3), retry.WithNextDelay(retry.Exponential(base)))

err := policy.Do(ctx, attempt) // same semantics as retry.Do
```

A `Policy` is immutable after construction and safe for concurrent use.

## Exponential backoff

`Exponential(baseDelay, opts...)` returns a `NextDelayFunc` computing `baseDelay × factor^attempt`, capped and widened by random jitter as configured
below. Build it once and reuse it: the returned function allocates nothing per call.

| Option                    | Default                           | Description                                              |
|---------------------------|-----------------------------------|----------------------------------------------------------|
| `WithExponentialMaxDelay` | unbounded                         | Cap on the computed delay; non-positive values are ignored |
| `WithExponentialFactor`   | `DefaultExponentialFactor` (1.5)  | Multiplier per attempt; zero means the default           |
| `WithExponentialJitter`   | 0 (deterministic)                 | Up to this fraction (0–1, clamped) of extra random delay |

```go
backoff := retry.Exponential(100*time.Millisecond,
    retry.WithExponentialMaxDelay(5*time.Second),
    retry.WithExponentialJitter(0.2),
)
```
