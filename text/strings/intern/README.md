# intern

```go
import "github.com/altessa-s/go-atlas-core/text/strings/intern"
```

Package `intern` provides lock-free string deduplication with LFU eviction (LRU tie-break). Two-tier cache: hot (atomic slots) + cold (`sync.Map`).
Reduces memory for repeated string values.

## Options

| Option      | Default        | Description                                                   |
|-------------|----------------|---------------------------------------------------------------|
| `WithClock` | `clock.Real()` | Time source for access timestamps (LRU tie-break on eviction) |

## Key types

| Function / Method                                                                                               | Description                                                         |
|-----------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------|
| `New`                                                                                                           | Create a sized interner (`DefaultMaxSize`, 8192, when non-positive) |
| `String`                                                                                                        | Deduplicate a string                                                |
| `LowerString` / `UpperString`                                                                                   | Case-convert and intern                                             |
| `TrimString`                                                                                                    | Trim whitespace and intern                                          |
| `StringSlice` / `StringMap`                                                                                     | Bulk intern                                                         |
| `PrefixString` / `SuffixString` / `WrapString` / `JoinString` / `FormatString` / `CleanPathString` / `JoinWith` | Build a string and intern the result                                |
| `Size` / `MaxSize` / `IsFull` / `LoadFactor` / `Reset`                                                          | Capacity inspection and full reset                                  |
| `Stats`                                                                                                         | Performance counters snapshot (hit/miss/eviction)                   |
| `ResetStats`                                                                                                    | Zero counters without clearing cache                                |

## Default interner

Every interning method above (`String` through `JoinWith`) is also available as a package-level function (`intern.String`, `intern.JoinString`, …)
that delegates to the process-wide interner returned by `Default()`. `ResetDefault()` clears it — call it from `t.Cleanup` in tests that use the
package-level functions.

## Performance monitoring

`Stats()` returns a `Stats` snapshot with atomic counters for hot hits, cold hits, misses, and evictions. Convenience methods compute hit rates:

```go
stats := interner.Stats()
fmt.Printf("hit rate: %.1f%%, hot: %.1f%%, size: %d/%d\n",
    stats.HitRate()*100, stats.HotHitRate()*100,
    stats.CurrentSize, stats.MaxSize)
```

`Stats()` exposes hot/cold hits, misses, evictions, current size, and max size as plain counters, so callers can export them to any metrics system.
