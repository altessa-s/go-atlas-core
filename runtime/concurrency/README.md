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
