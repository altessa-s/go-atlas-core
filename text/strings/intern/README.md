# intern

```go
import "github.com/altessa-s/go-atlas-core/text/strings/intern"
```

Package `intern` provides lock-free string deduplication with LFU eviction (LRU tie-break). Two-tier cache: hot (atomic slots) + cold (`sync.Map`).
Reduces memory for repeated string values. General string helpers live in the parent [strings](../README.md) package; secret handling (which must
never rely on interning) lives in the sibling [secure](../secure/README.md) package.

## How it works

A lookup checks the hot cache first (a lock-free `atomic.Pointer` read keyed by a hash of the string), then the cold `sync.Map`, then falls back
to creating a new `unique.Handle` entry. Promotion from cold to hot is evaluated once a string reaches `HotCacheThreshold` accesses and
re-evaluated every `PromotionCheckInterval` accesses after that; an occupied hot slot is taken over only when the candidate has more than twice
the occupant's access count, which avoids thrashing a slot between two moderately hot strings. Background eviction runs in its own goroutine once
the interner exceeds `MaxSize`, removing the least frequently accessed entries first and breaking ties by least recent access.

| Constant                 | Default | Description                                                                 |
|---------------------------|---------|------------------------------------------------------------------------------|
| `DefaultMaxSize`          | 8192    | Default interner capacity                                                   |
| `HotCacheSlots`           | 32      | Atomic pointer slots in the hot cache                                      |
| `HotCacheThreshold`       | 5       | Minimum access count before hot-cache promotion is considered              |
| `PromotionCheckInterval`  | 50      | Accesses per string between promotion re-evaluations                       |
| `EvictionBatchSize`       | 512     | Max entries removed per eviction pass                                     |
| `EvictionRatio`           | 4       | Up to 1/4 (25%) of the cache evicted per pass, capped by `EvictionBatchSize`, but never fewer than needed to get back under `MaxSize` |

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
