# slices

```go
import "github.com/altessa-s/go-atlas-core/collections/slices"
```

Package `slices` provides generic utilities for slice transformation, filtering, deduplication, grouping, ordering checks, conditional appends,
parallel processing, and pooling. It complements the standard library `slices` package with patterns that recur throughout the codebase.

Pure functions return new slices without modifying inputs — the exceptions are `Delete`, which removes in place like stdlib `slices.Delete` and
returns the shortened input slice, and the `Append*` / `EnsureCapacity` helpers, which grow the slice they are given. Nil slices are treated as
empty and never cause panics; results are also nil when empty, so `len(result) == 0` is the only check callers need. Closures passed as predicates
or transforms are never copied, so capturing state in them is safe. `Pool` is safe for concurrent use; `FilterParallel` and `MapParallel` invoke the
predicate or transform from multiple goroutines, which must therefore be safe for concurrent invocation.

> **Name collision:** this package is named `slices`, same as the standard library package. A file that imports both needs an alias on one of
> them — `coreslices` is a reasonable choice — to avoid a compile error.

## Functions

| Function          | Description                                                             |
|--------------------|-------------------------------------------------------------------------|
| `Deduplicate`      | Remove duplicates, preserving first-occurrence order                    |
| `DeduplicateBy`    | Remove duplicates using a custom key function                           |
| `Delete`           | Remove the first occurrence of an element                               |
| `To`               | Transform each element via a mapping function                          |
| `ToWithFilter`     | Filter and transform in a single pass — no intermediate slice           |
| `ToAny`            | Box a typed slice into `[]any`                                          |
| `ToStrings`        | Extract string elements from `[]any`, skipping non-strings              |
| `FilterFirst`      | Return the first element matching a predicate                           |
| `FilterLast`       | Return the last element matching a predicate                            |
| `FilterParallel`   | Filter using multiple goroutines for large slices                       |
| `MapParallel`      | Transform using multiple goroutines for large slices                    |
| `Reduce`           | Fold a slice into a single value with an accumulator                    |
| `GroupBy`          | Partition elements into groups by a key function                       |
| `All`              | True if every element matches a predicate                               |
| `AppendIf`         | Conditionally append values                                             |
| `AppendIfFunc`     | Conditionally append with lazy evaluation                               |
| `AppendNonEmpty`   | Append a key-value pair when the value is non-empty                     |
| `AppendNonNil`     | Append a value when it is non-nil                                       |
| `AppendNonNilErr`  | Append with lazy evaluation and error propagation                       |
| `ToList`           | Convert a slice to `container/list.List`                                |
| `FromList`         | Convert a `container/list.List` to a typed slice                        |

### Deduplication

`Deduplicate` and `DeduplicateBy` return a new slice with duplicates removed, preserving first-occurrence order. `Deduplicate` is a thin wrapper
around `DeduplicateBy` using an identity key function; both share the same code path.

```go
nums := []int{1, 2, 2, 3, 1, 4}
unique := slices.Deduplicate(nums) // [1, 2, 3, 4]

people := []Person{{1, "Alice"}, {2, "Bob"}, {1, "Alice2"}, {3, "Charlie"}}
unique := slices.DeduplicateBy(people, func(p Person) int { return p.ID })
// [{1, "Alice"}, {2, "Bob"}, {3, "Charlie"}]
```

Both are copy-on-write: the input slice is returned unchanged when no duplicates are found, and the result slice is allocated only once the first
duplicate is detected. A `lastKey` cache catches consecutive duplicates without a map lookup, which is the common case for sorted inputs. The
seen-set map is pre-sized to `min(len(collection), 128)` so very large slices with frequent duplicates do not over-allocate it.

### Removal

`Delete` removes the first occurrence of an element, returning the original slice unchanged when the element is absent. It locates the index with
`slices.Index` and delegates to `slices.Delete`, so the cost is `O(n)` plus a single shift.

```go
words := []string{"a", "b", "c"}
result := slices.Delete(words, "b") // ["a", "c"]
```

For removing every occurrence, use stdlib `slices.DeleteFunc` instead.

### Transformation

```go
nums := []int{1, 2, 3}
strs := slices.To[[]int, int, string](nums, strconv.Itoa) // ["1", "2", "3"]

evenStrs := slices.ToWithFilter(nums,
    func(n int) bool { return n%2 == 0 },
    func(n int) string { return fmt.Sprintf("even:%d", n) },
) // ["even:2"]
```

`To` and `ToWithFilter` use `~[]E` constraints so named slice types (`type IDs []int`) pass through without explicit conversion. `ToWithFilter` is
the canonical way to chain a filter and a transform without paying for an intermediate slice — roughly twice as fast as `Filter` followed by `To`
for non-trivial sizes, since it walks the input exactly once. The result is `nil` when no element passes, matching the `FilterMap` convention in
the sibling [`maps`](../maps/README.md) package.

### Filtering

```go
nums := []int{1, 2, 3, 4, 5}
firstEven, ok := slices.FilterFirst(nums, func(n int) bool { return n%2 == 0 }) // 2, true
lastEven, _ := slices.FilterLast(nums, func(n int) bool { return n%2 == 0 })    // 4, true
```

`FilterFirst` and `FilterLast` are zero-allocation and return as soon as a match is found (`FilterFirst` scans forward, `FilterLast` backward via
`slices.Backward`). Use stdlib `slices.IndexFunc` instead when only the index is needed, and the lazy `Filter` iterator (below) when every match is
needed and the result feeds into a pipeline.

#### `FilterParallel`

Below `32 * runtime.NumCPU() * 2` elements (for example ~512 on an 8-core machine) `FilterParallel` falls back to the sequential lazy `Filter`
iterator — no goroutine overhead. At or above that threshold it splits the input into `NumCPU` contiguous chunks, filters each chunk with its own
goroutine (`sync.WaitGroup.Go`, one plain sequential loop per chunk — no shared worker pool or dispatch channel), and concatenates the per-chunk
results in chunk order with `slices.Concat`, so element order is preserved. It has no `context.Context` parameter and is therefore not cancellable.
The predicate must be safe for concurrent invocation.

### Aggregation

```go
sum := slices.Reduce(nums, 0, func(acc, _ int, val int) int { return acc + val })

people := []Person{{"Alice", 30}, {"Bob", 25}, {"Charlie", 30}}
groups := slices.GroupBy(people, func(p Person) int { return p.Age })
// {25: [Bob], 30: [Alice, Charlie]}

hasEven := slices.ContainsFunc(nums, func(n int) bool { return n%2 == 0 }) // stdlib
allEven := slices.All(nums, func(n int) bool { return n%2 == 0 })
```

`Reduce` is zero-allocation and visits every element in order. `GroupBy` pre-sizes its internal map to `max(len(collection)*0.75, 16)` to reduce
rehashing as groups fill. `All` short-circuits on the first non-match and returns `true` for an empty slice (vacuous truth, matching Go convention);
use stdlib `slices.ContainsFunc` for an "any" check.

## Ordering

| Function               | Description                   |
|-------------------------|-------------------------------|
| `IsStrictlyIncreasing` | Every adjacent pair satisfies `a < b`  |
| `IsStrictlyDecreasing` | Every adjacent pair satisfies `a > b`  |
| `IsNonDecreasing`      | Every adjacent pair satisfies `a <= b` |
| `IsNonIncreasing`      | Every adjacent pair satisfies `a >= b` |

```go
slices.IsStrictlyIncreasing([]int{1, 2, 3}) // true
slices.IsNonDecreasing([]int{1, 2, 2, 3})   // true (equal values allowed)
```

All four take any `cmp.Ordered` element type and short-circuit on the first violation. An empty or single-element slice satisfies every check
vacuously.

## Conditional append

The `Append*` family replaces `if cond { s = append(s, x) }` call sites, keeping option/builder chains expression-shaped.

| Function          | When to use                                                              |
|--------------------|---------------------------------------------------------------------------|
| `AppendIf`         | Cheap value(s) guarded by a `bool`                                       |
| `AppendIfFunc`     | Non-trivial construction that should not run when the condition is false |
| `AppendNonEmpty`   | String-keyed pair lists where the value must be non-empty                |
| `AppendNonNil`     | Guard against typed-nil interfaces and nil pointers                      |
| `AppendNonNilErr`  | Lazy construction with error propagation                                 |

```go
opts := []Option{WithFoo(cfg.Foo)}
opts = slices.AppendIf(opts, cfg.Verbose, WithVerbose())
opts = slices.AppendIfFunc(opts, cfg.HasTLS(), func() []Option {
    return []Option{WithTLS(cfg.TLS.Cert, cfg.TLS.Key)}
})

attrs := []any{"trace_id", traceID}
attrs = slices.AppendNonEmpty(attrs, "user", req.User)

handlers := []Handler{authHandler}
handlers = slices.AppendNonNil(handlers, optionalMetricsHandler())
```

`AppendIf` evaluates its variadic values eagerly; reach for `AppendIfFunc` when evaluation is expensive or unsafe (for example dereferencing a
maybe-nil pointer), since it only calls `fn` after `cond` is checked. `AppendNonNil` checks nilness with
[`nilcheck.IsNil`](../../types/nilcheck/README.md), which correctly distinguishes a typed-nil interface (`var err error = (*pathError)(nil)`) from
a true nil — a plain `value != nil` does not catch that case. The plain `if` form is still fine when the branch contains additional logic that
doesn't fit a single `Append*` call.

## List interop

`container/list.List` adapters for the rare case where `list.List` shows up at an API boundary:

```go
nums := []int{1, 2, 3}
l := slices.ToList(nums)        // *list.List

back := slices.FromList[int](l) // [1, 2, 3]
```

`FromList` performs a per-element type assertion to `T` and silently skips elements that don't match. Use the `List` iterator (below) to walk a
`*list.List` lazily without building a slice at all.

## Iterators

Lazy `iter.Seq` iterators for zero-allocation pipelines (Go 1.23+). Construction allocates nothing and iteration is lazy, so early termination is
free. Use `slices.Collect()` to materialize results into a concrete slice on demand.

| Iterator    | Description                                        |
|-------------|-----------------------------------------------------|
| `Filter`    | Yield elements matching a predicate                 |
| `Map`       | Yield transformed elements                          |
| `List`      | Yield typed elements from a `container/list.List`   |
| `FilterSeq` | Filter an existing `iter.Seq`                       |
| `MapSeq`    | Transform an existing `iter.Seq`                    |
| `Take`      | Yield at most N elements from an `iter.Seq`         |

```go
nums := []int{1, 2, 3, 4, 5}

for n := range slices.Filter(nums, isEven) {
    // ...
}

// Compose a pipeline with no intermediate slices; stdslices is the standard
// library "slices" package, imported under an alias to avoid colliding with
// this one.
seq := slices.MapSeq(
    slices.FilterSeq(stdslices.Values(nums), isEven),
    func(n int) string { return fmt.Sprintf("n=%d", n) },
)
result := stdslices.Collect(slices.Take(seq, 100))
```

`Values`, `Backward`, and `Chunk` come from the standard library `slices` package rather than being re-implemented here. `Backward` yields an
`iter.Seq2[int, T]` so the index matches the original position, not the reverse-iteration step.

## Pool

`Pool[T]` (`NewPool(defaultCap)`, default capacity 64) is a generic, concurrency-safe `sync.Pool` wrapper for reusing `[]T` slice buffers in hot
paths.

```go
var bufPool = slices.NewPool[byte](256)

func handle(req *Request) {
    bufPtr := bufPool.Get()
    defer bufPool.Put(bufPtr)

    *bufPtr = append(*bufPtr, req.Body...)
    process(*bufPtr)
}
```

- **`Get`** resets the slice length to zero while preserving capacity; the cold path (pool empty) allocates `make([]T, 0, defaultCap)`.
- **`GetWithCapacity(n)`** replaces the pooled slice with a freshly allocated one when `cap(*slice) < n` and `n <= 1024`. Use it when an approximate
  upper bound is known, to avoid grow-and-copy churn during `append`.
- **`Put`** calls the built-in `clear` on the slice (so referenced values can be collected), resets the length, and returns it to the pool. Slices
  with `cap > 1024` are discarded so the pool does not retain runaway buffers.
- **Sentinel value** — a zero or negative `defaultCap` passed to `NewPool` is replaced with 64.

Always pair `Get` with `defer Put`, and never hand a pooled slice to another goroutine — ownership is bounded by the `Get`/`Put` pair. Pooling has
non-trivial overhead on cold paths; reach for it only once allocation profiling identifies a hot path that repeatedly constructs and discards the
same shape of slice. Avoid pooling slices of pointer-heavy structs without considering the GC: `clear` drops the slice's own references, but the
values they pointed to may stay live until the next pool turnover.

### `EnsureCapacity`

Standalone helper for growing a slice to a required capacity, rounding up to the next power of two for amortized append efficiency:

```go
buf := make([]int, 0, 4)
slices.EnsureCapacity(&buf, 100) // grows to capacity 128
```

Returns `true` when a new backing array was allocated, `false` when the existing capacity was already sufficient. The power-of-two rounding applies
only up to 1024 (the same cap `Pool.Put` uses to discard oversized slices); requests above that get exactly the requested capacity.

## Performance

- `Deduplicate` / `DeduplicateBy` are copy-on-write with a bounded (128-entry) seen-set pre-allocation and a consecutive-duplicate fast path — see
  [Deduplication](#deduplication).
- `FilterFirst`, `FilterLast`, `All`, `Reduce` are zero-allocation and short-circuit on the first match or failure (`Reduce` always visits every
  element).
- `MapParallel` and `FilterParallel` switch from sequential to parallel at `32 * runtime.NumCPU() * 2` elements (e.g. ~512 on 8 cores); below that,
  sequential execution avoids goroutine setup cost entirely. Both preserve input order and require a concurrency-safe callback.
- `MapParallel` splits work into one fixed, contiguous chunk per core via `sync.WaitGroup.Go`, with no worker pool or dispatch channel — routing
  the same work through a shared pool measured 13-95% slower (for sizes 1e3-1e5) with eight more allocations per call.
- `FilterParallel` follows the same fixed-chunk-per-goroutine approach; because each goroutine may append a variable number of matches, chunk
  results are collected into per-chunk slices and concatenated afterward rather than written in place.
- `GroupBy` pre-sizes its internal map to `max(len(collection)*0.75, 16)` so the typical fill rarely triggers a rehash.
- `ToAny`, `To`, `MapParallel` allocate the output slice exactly once, sized to the input length.
- `Pool.Put` clears the slice with the built-in `clear` (Go 1.21+) so pooled buffers do not pin old element values past their useful lifetime.
- `EnsureCapacity` rounds up to the next power of two below the 1024 cap, matching the runtime's growth heuristic for amortized `O(1)` appends.

## Best practices

- **Reach for the iterator variants when chaining.** `Filter` / `Map` / `FilterSeq` / `MapSeq` avoid an intermediate slice per pipeline stage; the
  materialized variants (`To`, `ToWithFilter`, `FilterParallel`, …) exist for when the result itself is the artifact (a cache value, a return type,
  a JSON payload).
- **Prefer `ToWithFilter` over `Filter` + `To`** when both a filter and a transform are needed — it walks the input once instead of twice.
- **Pick the narrowest tool:** `FilterFirst` / `FilterLast` when only the match is needed (zero allocation, early return), stdlib `slices.IndexFunc`
  when only the index is needed, `Filter` when every match is needed and will be chained further.
- **Don't second-guess the `MapParallel` / `FilterParallel` threshold** without a benchmark — it was set so goroutine setup cost is amortized over
  enough work to pay back.
- **Use the `Append*` helpers in factory/option chains** instead of the imperative `if cond { s = append(s, x) } ` form; keep the imperative form
  only when the branch needs additional logic that doesn't fit a single `Append*` call.
- **Treat `nil` and empty as equivalent.** Every function here treats a nil slice input as empty and returns nil for an empty result, so
  `len(result) == 0` is the only check most callers need.

## See also

- [`maps`](../maps/README.md) — companion package with the same conventions for map operations.
- [`nilcheck`](../../types/nilcheck/README.md) — the nil-detection logic behind `AppendNonNil`.
- [`runtime/concurrency`](../../runtime/concurrency/README.md) — a general-purpose bounded-concurrency fan-out, independent of the fixed
  NumCPU chunking `FilterParallel` / `MapParallel` use internally; reach for it when fan-out needs cancellation or a worker pool.
</content>
