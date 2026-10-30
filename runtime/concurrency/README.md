# concurrency

```go
import "github.com/altessa-s/go-atlas-core/runtime/concurrency"
```

Package `concurrency` provides adaptive concurrency limit calculation, a concurrent batch processor for parallelizing work across slices, and
ordered failover across named sources.

## Batch processing

| Function         | Description                                             |
|------------------|---------------------------------------------------------|
| `Process`        | Apply a function to every item with bounded concurrency |
| `ProcessCollect` | Same as `Process` but collect transformed results       |

```go
err := concurrency.Process(ctx, userIDs, func(ctx context.Context, id string) error {
    return syncUser(ctx, id)
},
    concurrency.WithConcurrency[string](8),
    concurrency.WithStopOnError[string](),
)
```

`ProcessCollect` preserves input order in the result slice by writing directly into a pre-allocated array indexed by position (no mutex, no sort
step):

```go
results, err := concurrency.ProcessCollect(ctx, urls, func(ctx context.Context, url string) (Response, error) {
    return httpGet(ctx, url)
},
    concurrency.WithConcurrency[string](4),
)
// On err == nil: len(results) == len(urls) and results[i] corresponds to urls[i].
```

When concurrency resolves to 1 (or the slice has a single element), both functions fall back to sequential execution in the caller's goroutine — no
goroutine, channel, or worker-pool overhead.

### Error handling

- `WithStopOnError()` cancels the internal context on the first error; in-flight items may still finish, but no new items start. Only the first
  error is returned.
- Without `WithStopOnError`, every item is still processed; the first error encountered is still the one returned.
- `WithOnSuccess` / `WithOnError` callbacks are invoked under a mutex, so mutating shared state from them is safe without external synchronization.
- **`ProcessCollect` returns a result only for items that succeeded.** When any item's function returns an error, the result slice is compacted to
  the completed items — order is preserved among the survivors, but the slice is shorter than the input and its indices no longer match `items`.

That last rule decides how to signal failure from a `TransformFunc`. Returning `(value, error)` tells `ProcessCollect` to *discard* `value` on
failure — correct for a fetch, where a failed item has no result worth keeping, and wrong whenever the returned value is itself the outcome you
meant to persist (e.g. an item whose status you are updating in place). When the value must survive a failure, carry the outcome **in** the value
and return a nil error instead:

```go
// The transform cannot fail; the verdict travels on the item itself.
results, _ := concurrency.ProcessCollect(ctx, events, func(ctx context.Context, e Event) (Event, error) {
    if err := dispatch(ctx, e); err != nil {
        e.MarkFailed(err) // Recorded, and still collected.
        return e, nil
    }
    e.MarkSent()
    return e, nil
})
```

## Waiting

| Function      | Description                                                          |
|---------------|----------------------------------------------------------------------|
| `WaitContext` | Wait for a `sync.WaitGroup` until a context ends (its error if first) |

## Ordered failover

| Symbol         | Description                                                                                 |
|----------------|---------------------------------------------------------------------------------------------|
| `First`        | Try `Source[T]` values in order; return the first value the `accept` policy admits          |
| `Source[T]`    | A named candidate: `Name` plus `Fetch func(ctx) (T, error)`                                 |
| `Result[T]`    | The admitted `Value`, its `Source` name and every skipped source in `Skipped`               |
| `SourceError`  | One skipped source (`Source`, `Err`): a fetch failure or an accept rejection                |
| `ErrAllFailed` | Joined with every `SourceError` when all sources are skipped; returned alone for no sources |
| `Canceled`     | Reports whether an error ended work because the context is done, not because of a failure   |

`First` checks the context before each source and after a failed fetch. A done context is returned as `ctx.Err()`, not joined with `ErrAllFailed`, and
the interrupted source is not recorded. A fetch that times out on its own deadline while the caller's context is alive is an ordinary skip.
`Result.Skipped` is filled on every path, so callers can count failures even when `First` fails.

```go
res, err := concurrency.First(ctx, []concurrency.Source[Snapshot]{
    {Name: "primary", Fetch: primary.Fetch},
    {Name: "mirror", Fetch: mirror.Fetch},
}, func(source string, s Snapshot) error { return validate(s) })
switch {
case concurrency.Canceled(ctx, err):
    return err // shutdown or caller deadline
case err != nil:
    return fmt.Errorf("snapshot: %w", err) // errors.Is(err, concurrency.ErrAllFailed)
}
```

## Options

| Option            | Description                                                  |
|-------------------|--------------------------------------------------------------|
| `WithConcurrency` | Fixed concurrency limit (number of worker goroutines)        |
| `WithLimitFunc`   | Dynamic limit via a `ConcurrencyLimitFunc` strategy          |
| `WithStopOnError` | Cancel remaining work after the first error                  |
| `WithOnSuccess`   | Callback invoked after each successfully processed item      |
| `WithOnError`     | Callback invoked with the item and error after each failure  |

A positive `WithLimitFunc` result takes precedence; otherwise a positive `WithConcurrency` value is used, and with neither the limit falls back to
`DefaultLimitFunc`.

## Concurrency strategies

| Factory                          | Strategy                                                    |
|----------------------------------|-------------------------------------------------------------|
| `ConcurrencyForEnvironment`      | Preset limits by workload profile                           |
| `MemoryAwareConcurrency`         | Scale by remaining memory budget                            |
| `LoadAwareConcurrency`           | Scale by system load average                                |
| `AdaptiveConcurrency`            | Combine memory + load factors (thresholds, load source)     |
| `ConnectionPoolAwareConcurrency` | Limit by available pool connections                         |
| `DefaultLimitFunc`               | `NumCPU * 2` (I/O-bound default)                            |

## Memory budgets

`MemoryAwareConcurrency` and `AdaptiveConcurrency` use the remaining Go runtime budget configured through `GOMEMLIMIT` or `debug.SetMemoryLimit`. The
remaining bytes are `max(0, limit - (MemStats.Sys - MemStats.HeapReleased))`; statistics are cached for `DefaultMemStatsCacheTTL` (one second) in an
internal, fixed cache. For another TTL or a test clock, build a `MemStatsCache` with `NewMemStatsCache(ttl, clk)` and pass
`WithAvailableMemory(cache.RuntimeHeadroom)`; `cache.Invalidate()` forces a refresh. This measures runtime headroom, not free machine RAM. Without a
finite budget, the memory strategy uses `DefaultLimitFunc` and adaptive mode skips memory scaling.

Pass `WithAvailableMemory(source)` to either strategy to supply a thread-safe `MemorySource` for a container or host budget. It returns available bytes
and whether the budget is known. Returning `(0, true)` means exhausted; `(0, false)` means unknown. Callbacks should cache expensive probes.

## Environment presets

| Environment                    | Concurrency           |
|--------------------------------|-----------------------|
| `EnvironmentMemoryConstrained` | 1                     |
| `EnvironmentCPUBound`          | `NumCPU`              |
| `EnvironmentIOBound`           | `NumCPU * 2`          |
| `EnvironmentHighThroughput`    | `NumCPU * 4`          |
| `EnvironmentRateLimited`       | 3                     |

## Patterns and guidelines

### Prefer `Process` over manual goroutine management

Instead of hand-writing `sync.WaitGroup` + semaphore + error-collection boilerplate, use `Process`/`ProcessCollect`. They handle cancellation, error
propagation, callbacks, and the sequential fallback automatically.

### Choose the right concurrency limit

- **I/O-bound** (HTTP calls, database queries): `ConcurrencyForEnvironment(EnvironmentIOBound)` or `MemoryAwareConcurrency`.
- **CPU-bound** (hashing, compression): `ConcurrencyForEnvironment(EnvironmentCPUBound)`.
- **External APIs with rate limits**: `ConcurrencyForEnvironment(EnvironmentRateLimited)` or `ConnectionPoolAwareConcurrency`.
- **Variable load**: `AdaptiveConcurrency` with both memory and load thresholds.

### Use `StopOnError: false` for non-critical fan-out

When checking multiple independent resources (health checks, leader-election callbacks, secret warmup), omit `WithStopOnError` so every item gets a
chance to run even when some fail; the first error is still returned:

```go
err := concurrency.Process(ctx, checks, runCheck,
    concurrency.WithConcurrency[Check](4),
    // No WithStopOnError — check all services even if one is unhealthy.
)
```

### Reuse timers in loops

Avoid `time.After` in loops — since Go 1.23 an unreferenced timer is garbage collected before it fires, but each call still allocates a new timer.
Create one `time.Timer` per loop and `Reset` it each iteration instead:

```go
timer := time.NewTimer(interval)
defer timer.Stop()

for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        doWork()
        timer.Reset(interval)
    }
}
```

With `go 1.23` or later in `go.mod`, timer channels are synchronous: after `Stop` or `Reset` returns, no stale value is received, so no manual drain
is needed.

### Use atomics for concurrent state flags

Prefer `atomic.Bool` for running-state flags, `atomic.Int64` for counters, and `atomic.Int32` for gauges — they are cheaper than a mutex for
single-value access and make concurrent access explicit:

```go
type Coordinator struct {
    activeWatchers    atomic.Int32 // gauge: current subscription count
    listCallsInFlight atomic.Int32 // gauge: in-progress list calls
}
```

Increment with `Add(1)`, decrement with `Add(-1)`, and read with `Load()`. Keep such a field only while something reads it — an atomic that is
faithfully incremented and decremented but never loaded is not instrumentation, it is cost with no signal.

### Prevent concurrent execution with single-flight guards

Use an `atomic.Bool` with `CompareAndSwap` to ensure only one goroutine runs a periodic cycle at a time. This is lighter than a mutex when the goal
is to skip overlapping invocations rather than queue them:

```go
var running atomic.Bool

func runCycle(ctx context.Context) error {
    if !running.CompareAndSwap(false, true) {
        return nil // a cycle is already in flight; skip this one
    }
    defer running.Store(false)
    return doCycleWork(ctx)
}
```

For request deduplication by key (coalescing concurrent callers rather than skipping them), use
[`runtime/singleflight`](../singleflight/README.md) instead — see its README for `Do`, `WithTimeout`, and `WithMaxConcurrent`.
