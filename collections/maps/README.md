# maps

```go
import coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
```

Package `maps` provides generic utilities for map transformation, filtering, conversion, pooling, and weak references. All pure functions return new
maps without modifying inputs; nil maps are treated as empty and never cause panics.

> **Import alias:** the package name `maps` collides with the standard library `maps` package. Always import it as `coremaps`, as used throughout
> this module.

## Functions

| Function                 | Description                                                |
|--------------------------|------------------------------------------------------------|
| `ConvertMap`             | Transform keys and values via a mapping function           |
| `FilterMap`              | Return entries matching a predicate                        |
| `FromFlatMap`            | Expand dot-separated keys into nested maps                 |
| `FromFlatMapWithHandler` | Same as `FromFlatMap` with a conflict callback             |
| `FromSlice`              | Build a map from a slice using a key extractor             |
| `FromSliceWith`          | Build a map from a slice using a key-value extractor       |
| `Merge`                  | Combine two maps; source keys take precedence              |
| `MergeAll`               | Overlay N maps in order; last layer wins                   |
| `MergeDeep`              | Recursively merge nested `map[string]any` structures       |
| `MergeWith`              | Combine two maps with a custom conflict resolver           |
| `Swap`                   | Transpose keys and values                                  |
| `ToFlatMap`              | Flatten nested maps into dot-separated keys                |
| `ToKeyValueSlice`        | Flatten a map to `[]any{k1, v1, k2, v2, ...}`              |
| `ToSortedKeyValueSlice`  | Flatten alternating keys and values in ascending key order |

`ToSortedKeyValueSlice` returns alternating keys and values sorted by key.
`ToKeyValueSlice` has random iteration order. Both return nil for empty input.

`FromFlatMap` processes keys in lexical order: scalar parents block their descendants, while explicit dotted entries override values inside map
parents. Nested `map[string]any` values are copied before expansion, including nil maps; other reference values remain shared. `ToFlatMap` preserves
empty key segments, including those produced by a key mapper. The hot loop uses `strings.IndexByte` instead of `strings.Split`, so expanding a key
allocates no intermediate slices.

When a key path traverses through a non-map value, the conflicting entry is silently skipped; `FromFlatMapWithHandler`'s callback exists purely for
observability (logging or counting conflicts) — the entry is skipped regardless of what the handler does.

## Iterators

Lazy `iter.Seq` / `iter.Seq2` iterators for zero-allocation pipelines. Use stdlib `maps.Collect()` to materialize results into concrete map values.

| Iterator | Description                        |
|----------|-------------------------------------|
| `Filter` | Yield entries matching a predicate |
| `Map`    | Yield transformed key-value pairs  |

Prefer `Filter` / `Map` over `FilterMap` / `ConvertMap` whenever the result is consumed exactly once — chaining stays allocation-free until the final
`maps.Collect`:

```go
// Single allocation at the end.
out := maps.Collect(coremaps.Filter(m, isHot))

// Worse: FilterMap allocates its own intermediate result map.
out := coremaps.FilterMap(m, isHot)
```

Reach for `FilterMap` / `ConvertMap` when the result itself is the artifact you need to keep (a cache value, a return type, a JSON payload).

## Pool

`Pool[K, V]` (`NewPool(defaultCap)`, default capacity 64) is a generic, concurrency-safe `sync.Pool` wrapper for reusing map allocations. `Get` /
`GetWithCapacity` hand out a cleared `*map[K]V`; `Put` returns it, discarding maps exceeding 1024 entries.

`Get` clears the map before handing it out, so callers always receive an empty map; if the pool is empty, a fresh map of `defaultCap` capacity is
allocated. `GetWithCapacity(n)` reuses the pooled allocation directly when `n` fits within `defaultCap`; larger requests discard the pooled map and
allocate a freshly sized one, since Go does not expose a map's underlying capacity, so a previously grown pooled map can't be distinguished from a
baseline-sized one. `Put` clears the map and discards it instead of re-pooling once `len(*m) > 1024`, so the pool doesn't retain runaway
allocations; this also makes `Get` O(1) since the clear cost is paid by `Put`, not the next `Get`.

Best practices:

- **Always pair with `defer Put`** — leaking a pooled map back to the GC defeats the purpose.
- **Don't share** — never hand a pooled map to another goroutine; ownership is bounded by `Get` / `Put`.
- **Pool in hot paths only** — `sync.Pool` overhead is non-trivial for cold paths. Pool only when profiling identifies a hot path that repeatedly
  constructs and discards the same shape of map.
- **Don't exceed 1024 entries** in steady state, or the pool drops everything you return and effectively becomes a no-op.

## WeakMap

`WeakMap[K, V]` (`NewWeakMap`) holds weak references to values via `Get` / `Set` / `Delete` / `Range` / `Len`. Entries are automatically removed when
the GC reclaims the value via `runtime.AddCleanup` — no periodic sweeps, manual eviction, or background goroutines needed; `Len` is approximate, so
call `Cleanup` first for an exact count. Safe for concurrent access.

`Set(key, nil)` behaves like `Delete`. Each entry owns exactly one cleanup: replacing the value of a key stops the previous value's cleanup, `Delete`
stops it too, and setting the value a key already holds is a no-op. A cleanup that was already queued when its entry was replaced or deleted checks the
entry's generation and never removes the newer entry. `Range` takes a snapshot of all live entries under a write lock and then iterates without holding
any lock, so the callback may safely call other `WeakMap` methods without deadlocking.

When to use:

- Caches where a separate strong reference (e.g. a request handler holding a `*Session`) determines lifetime, and the cache should not extend it.
- Indices into a primary collection where the primary collection can drop entries at any time.
- Per-object metadata that should disappear with the object.

When NOT to use:

- Any cache whose hit rate must be predictable — GC timing is not.
- Caches where eviction must run on a clock (TTLs); use a dedicated TTL-based cache instead. Mixing the two policies surprises future readers.

## WeakRef

`WeakRef[T]` (`MakeWeakRef`) is a thin wrapper around `weak.Pointer` for holding a single weak reference. Use `IsAlive` to check and `Value` to
retrieve. The zero value behaves as if the referenced object has already been collected. `IsAlive`'s result may be stale immediately after the call
returns, so prefer checking `Value() != nil` and holding on to the returned pointer for the duration of the operation rather than calling `IsAlive`
and then `Value` separately.

## ImmutableMap

`ImmutableMap[K, V]` is a read-only hash map that cannot be modified after construction. Build once from a `map[K]V` or parallel key/value slices,
then read concurrently without locks or defensive copies. Uses a Swiss-table layout with flat arrays for lower memory overhead (~1.5–2× savings at
scale) and reduced GC scan work compared to a standard Go map when `K` and `V` are pointer-free (pointer-holding keys or values such as `string` are
still scanned).

```go
src := map[string]int{"a": 1, "b": 2, "c": 3}
m := coremaps.NewImmutableMap(src)
v, ok := m.Get("a") // 1, true
```

| Method                       | Description                                        |
|-------------------------------|----------------------------------------------------|
| `NewImmutableMap`            | Build from a standard Go map                       |
| `NewImmutableMapFromEntries` | Build from parallel key and value slices; panics on length mismatch |
| `Get`                        | Lookup by key; returns value and presence flag      |
| `Contains`                   | Check key presence                                  |
| `Len`                        | Number of entries                                   |
| `All`                        | Iterator over all key-value pairs (`iter.Seq2`)     |
| `Keys`                       | Iterator over keys (`iter.Seq`)                     |
| `Values`                     | Iterator over values (`iter.Seq`)                   |

`NewImmutableMap` copies from the source map, which is not retained and may be reused or discarded by the caller afterward. Iteration order via
`All` / `Keys` / `Values` is hash-dependent and not guaranteed.

### Why use it

| Property                  | Standard `map`                                                        | `ImmutableMap`                                               |
|----------------------------|------------------------------------------------------------------------|---------------------------------------------------------------|
| Concurrent reads          | Safe only with `sync.RWMutex` or under build-once-publish discipline  | Always safe, no lock                                          |
| Memory overhead at scale  | Pointer-heavy bucket layout                                           | Three flat slices; ~1.5–2× lower memory at >10K entries       |
| GC scan cost              | Walks every bucket pointer                                            | `keys`/`vals` not scanned when `K` and `V` are pointer-free   |
| Allocation count          | Grows with rehashing                                                  | Exactly 4 heap allocations regardless of size                |
| Lookup speed              | Comparable                                                            | Comparable on Go 1.24+ (also Swiss-table internally)          |
| Mutation                  | Yes                                                                    | None — `Set`/`Delete` do not exist                            |

### When to use

- Package-level lookup tables: `var codes = coremaps.NewImmutableMap(map[string]struct{}{…})`
- Struct fields populated in a constructor and never modified (registries, codec mappings, field schemas)
- Configuration maps that are built from options and frozen before use
- Any map you would otherwise comment with `// read-only` or `// do not modify`

### When NOT to use

- Maps that grow lazily at runtime (caches, `sync.Map`-backed stores)
- Maps modified after construction (use `sync.RWMutex` + plain `map` or `sync.Map`)
- Ephemeral maps local to a function — plain `map` has zero ceremony cost

### Freeze pattern

When a type has a mutable registration phase followed by a long read-only phase, store both a `map` (writes) and an `*ImmutableMap` (reads). Add a
`Freeze()` method that builds the `ImmutableMap` and nils the mutable map, and have the registration method panic if called after `Freeze`.

### How it works

`ImmutableMap` uses an open-addressing Swiss-table layout:

- **Group size 8** — control bytes are packed into one `uint64` per group; lookup uses SWAR (SIMD Within A Register) bit-tricks to scan all 8 lanes
  in a single integer comparison, no branches.
- **H2 fingerprint** — the low 7 bits of the hash are stored as a per-slot fingerprint; bit 7 distinguishes empty slots. This lets `Get` reject most
  candidate slots without touching the keys array.
- **87.5% target load factor** — the table is sized to `n*8/7` slots, rounded up to a multiple of 8. Higher density than the runtime map, with
  predictable probe sequences.
- **`hash/maphash` per-instance seed** — randomizes the layout so adjacent maps don't share collision patterns.

The data is laid out as three contiguous slices (`ctrl`, `keys`, `vals`) with no overflow chains or per-bucket pointers. When `K` and `V` are
pointer-free, the GC does not scan the backing arrays at all, so scan cost is O(1) regardless of population; when they contain pointers (e.g.
`string`, `*T`), the GC still walks every slot of `keys` / `vals`. `Get` is allocation-free: the SWAR `matchByte` runs in a few cycles and
short-circuits on the first empty slot in the probe sequence — early termination is safe because the table is build-once, so there are no
tombstones.

## Best practices

- **Reach for `ImmutableMap` for build-once-read-many.** If a map is constructed in a factory or `init` and never written afterward, `ImmutableMap`
  is the explicit lock-free contract — it documents the intent in the type itself, so there's no `// do not modify` comment to rot.
- **Treat `nil` and empty as equivalent.** Every function in this package treats a `nil` map input as empty. Returns are also `nil` when the result
  is empty, so `len(result) == 0` is the only check callers need.

## Performance notes

- `Merge`, `Swap`, `FilterMap`, `ConvertMap`, `FromSlice`, `FromSliceWith` pre-size the result map from the input length to avoid rehashing.
  `FilterMap` pre-sizes to `len/2` on the assumption that filters reject roughly half; past that, normal map growth kicks in, still cheaper than
  starting at `0`.
- `ToKeyValueSlice` pre-sizes the result to `len(m)*2` and appends in a single pass.
- `FromFlatMap` walks each key with `strings.IndexByte` and reuses the substring view, so there's no `strings.Split` allocation per key.
- `ImmutableMap` has exactly 4 heap allocations regardless of size (the struct plus the `ctrl`, `keys`, `vals` slices), versus the runtime map's
  growing bucket array.
- `WeakMap.Range` snapshots under a write lock so user code in the callback cannot deadlock against further `Set` / `Delete` calls.

## See also

- [slices](../slices/README.md) — companion package with the same conventions for slice operations.
- [runtime/concurrency](../../runtime/concurrency/README.md) — bounded fan-out for parallelizing per-entry work over a map.
