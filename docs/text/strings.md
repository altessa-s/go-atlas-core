# Strings

Concepts for working with `core/text/strings` (string manipulation, case transformation, zero-copy operations) and its subpackages
`core/text/strings/intern` (interning) and `core/text/strings/secure` (secure storage and timing-safe comparison).

```
import "github.com/altessa-s/go-atlas-core/text/strings"
```

All utility functions are pure and thread-safe. `secure.String` is safe for concurrent reads. `intern.Interner` is fully thread-safe with lock-free
design.

---

## Validation and emptiness

```go
strings.IsEmpty("")         // true
strings.IsEmpty("  ")       // true  (whitespace-only)
strings.IsEmpty[*string](nil) // true

strings.IsEmptyOrWhitespace("  \t") // true — allocation-free, rune-based
```

`IsEmpty` is generic over `~string | ~*string`. `IsEmptyOrWhitespace` iterates runes directly and returns on the first
non-space rune, instead of trimming the whole string with `strings.TrimSpace` (which does not allocate, but always scans both ends).

### Trimming checks

| Function | Description |
|----------|-------------|
| `IsTrimmed(s)` | No leading/trailing whitespace (ASCII fast path, Unicode fallback) |
| `IsTrimmedUnsafe(s)` | Same, but uses `ToBytesUnsafe` for zero-copy byte access |

---

## Pointer conversion

```go
ptr := strings.ToPtr("hello")  // &"hello"
ptr = strings.ToPtr("")         // nil (empty/whitespace → nil)
s := strings.FromPtr(ptr)       // "hello" (nil → "")
```

`ToPtr` is generic over `~string | ~*string`.

---

## Numeric conversion

Generic `To[T]` parses a string into any numeric type. Returns zero on parse error.

```go
n := strings.To[int64]("42")       // 42
f := strings.To[float64]("3.14")   // 3.14
```

Pre-instantiated converters avoid type parameters at call sites:

| Variable | Type |
|----------|------|
| `ToInt`, `ToInt64`, `ToInt32`, `ToInt16`, `ToInt8` | signed integers |
| `ToUint`, `ToUint64`, `ToUint32`, `ToUint16`, `ToUint8` | unsigned integers |
| `ToFloat32`, `ToFloat64` | floats |

---

## Case transformation

| Function | Example |
|----------|---------|
| `ToSnakeCase(s)` | `"HelloWorld"` → `"hello_world"` |
| `ToScreamingSnakeCase(s)` | `"HelloWorld"` → `"HELLO_WORLD"` |
| `ToCamelCase(s)` | `"hello_world"` → `"helloWorld"` |
| `ScreamingSnakeToCamelCase(s)` | `"HELLO_WORLD"` → `"helloWorld"` |

`ToCamelCase` lowercases only the first letter and keeps the case of every other letter that does not start a word, so `"SCREAMING_SNAKE"` yields
`"sCREAMINGSNAKE"`; `ScreamingSnakeToCamelCase` also lowercases the rest of each word (`"REAL_IP"` → `"realIp"`). Both treat underscores and Unicode
whitespace as word delimiters.

All four functions use an ASCII fast path with a Unicode (rune-based) fallback for non-ASCII input. The camelCase functions copy invalid UTF-8
bytes through unchanged.

### Case checks

| Function | Description |
|----------|-------------|
| `IsLowercase(s)` | All letters are lowercase (rune-based, allocation-free) |
| `IsUppercase(s)` | All letters are uppercase (rune-based, allocation-free) |
| `IsLowercaseUnsafe(s)` | ASCII fast path, Unicode fallback |
| `IsUppercaseUnsafe(s)` | ASCII fast path, Unicode fallback |

---

## Split and Join

### Split

`Split` divides a string with configurable options:

```go
parts := strings.Split("a,,b", strings.SplitOptions{
    Separator: ",",
    SkipEmpty: true,    // removes empty elements
    TrimSpace: true,    // trims whitespace from each part
    MaxSplits: 3,       // limits number of splits (-1 = no limit)
    CaseSensitive: false, // case-insensitive separator matching
})
// ["a", "b"]
```

Case-insensitive splitting matches the separator under Unicode simple case folding (the rule used by `strings.EqualFold`), so `"ſ"` matches `"s"`,
the Kelvin sign matches `"k"`, and `"Σ"`, `"σ"`, `"ς"` match each other. Pure ASCII input and separator use a fast lowercase scan; anything else
uses a cached case-insensitive regex (bounded at 256 entries). A separator that is not valid UTF-8 is matched exactly.

### SplitSeq (lazy iterator)

`SplitSeq` returns an `iter.Seq[string]` that yields exactly the parts `Split` returns but avoids allocating an intermediate slice. Suitable for large
inputs or early termination:

```go
for part := range strings.SplitSeq(largeCSV, strings.SplitOptions{Separator: ","}) {
    if part == target {
        break // no further work
    }
}
```

### Join

```go
result := strings.Join([]string{"a", "", "b"}, strings.JoinOptions{
    Separator: ",",
    SkipEmpty: true,   // "a,b" instead of "a,,b"
    Prefix:    "[",
    Suffix:    "]",
})
// "[a,b]"
```

---

## Contains (advanced search)

```go
r := strings.Contains("Hello World Hello", "hello", strings.ContainsOptions{
    CaseSensitive:   false, // case-insensitive
    MatchWholeWords: true,  // word boundaries only
    Count:           true,  // count all occurrences
})
// r.Found = true, r.Count = 2, r.Positions = [0, 12]
```

When `Count` is false, the search returns on the first match (fast path).

---

## Concatenation

| Function | Description |
|----------|-------------|
| `Concat(parts...)` | Pooled `strings.Builder`, single allocation, pre-grown |
| `ConcatUnsafe(parts...)` | Single `[]byte` allocation + `FromBytesUnsafe` |

Both return the original string for single-element input and empty string for zero elements.

---

## Trim helpers

| Function | Description |
|----------|-------------|
| `TrimPrefixFast(s, prefix)` | Substring-based, never allocates |
| `TrimSuffixFast(s, suffix)` | Substring-based, never allocates |

These use direct substring comparison instead of `strings.TrimPrefix`/`TrimSuffix`.

---

## Zero-copy conversion

```go
b := strings.ToBytesUnsafe("hello")   // []byte — must NOT be modified
s := strings.FromBytesUnsafe(buf)      // string — source must NOT be modified after
```

Both use `unsafe.StringData`/`unsafe.SliceData` (Go 1.20+). The safety contract:
- `ToBytesUnsafe`: returned slice is read-only; modifying it is undefined behavior.
- `FromBytesUnsafe`: source slice must not be modified after conversion.

### Unsafe equality

`StringEqualsUnsafe` returns `false` immediately when lengths differ, then compares data pointers (O(1) for interned strings), then falls back to
standard `==`. The comparison is always case-sensitive — there is no `strings.EqualFold` fallback.

---

## String interning

The `intern.Interner` (`import "github.com/altessa-s/go-atlas-core/text/strings/intern"`) deduplicates identical strings so only one copy is retained in
memory. Uses a two-tier cache:

1. Hot cache: 32 `atomic.Pointer` slots for the most frequently accessed strings
   (wait-free reads).
2. Cold cache: `sync.Map` for general interning with `unique.Handle` storage.

Promotion is evaluated when a string reaches 5 accesses and re-evaluated every 50 accesses to that string: if the string qualifies, it
is promoted to the hot cache (an occupied slot is taken over only by a string with more than twice the occupant's access count). Background eviction
runs when the interner exceeds capacity; it removes the least frequently accessed entries first (LFU), breaking ties by least recent access (LRU).

### Default interner

```go
s := intern.String("/api/users")  // deduplicated via intern.Default()
```

| Convenience function | Description |
|---------------------|-------------|
| `String(s)` | Intern as-is |
| `LowerString(s)` | Lowercase + intern |
| `UpperString(s)` | Uppercase + intern |
| `TrimString(s)` | Trim whitespace + intern |
| `CleanPathString(s)` | Normalize path (collapse `//`, strip trailing `/`) + intern |
| `PrefixString(s, prefix)` | Prepend + intern |
| `SuffixString(s, suffix)` | Append + intern |
| `WrapString(s, prefix, suffix)` | Wrap + intern |
| `StringSlice(slice)` | Intern every element |
| `StringMap(m)` | Intern every key and value |
| `FormatString(fmt, args...)` | `Sprintf` + intern |
| `JoinString(parts, sep)` | Join + intern |
| `JoinWith(parts, joiner)` | Custom join + intern |

### Custom interner

```go
interner := intern.New(4096) // custom capacity
// intern.New(4096, intern.WithClock(fake)) injects the access-time source (default clock.Real())

s := interner.String("key")
s = interner.LowerString("KEY")
s = interner.CleanPathString("//foo//bar/")  // "/foo/bar"
```

### Performance monitoring

```go
stats := intern.Default().Stats()
fmt.Printf("hit rate: %.1f%% (hot: %.1f%%)\n",
    stats.HitRate()*100, stats.HotHitRate()*100)
fmt.Printf("size: %d/%d, evictions: %d\n",
    stats.CurrentSize, stats.MaxSize, stats.Evictions)
```

| `Stats` field | Description |
|----------------------|-------------|
| `HotHits` | Lookups served from hot cache |
| `ColdHits` | Lookups served from cold cache |
| `Misses` | New entries created |
| `Evictions` | Entries removed by background eviction |
| `CurrentSize` / `MaxSize` | Current and max capacity |

`ResetStats()` zeroes counters without clearing cached strings.

### Tuning constants

| Constant | Default | Description |
|----------|---------|-------------|
| `DefaultMaxSize` | 8192 | Default interner capacity |
| `HotCacheSlots` | 32 | Atomic pointer slots in hot cache |
| `HotCacheThreshold` | 5 | Minimum access count for hot promotion |
| `PromotionCheckInterval` | 50 | Accesses per string between promotion re-evaluations |
| `EvictionBatchSize` | 512 | Max entries per eviction pass |
| `EvictionRatio` | 4 | Up to 25% of cache evicted per pass (`size/4`, capped by `EvictionBatchSize`) |

---

## Secure strings

`secure.String` (`import "github.com/altessa-s/go-atlas-core/text/strings/secure"`) stores sensitive data (passwords, tokens, keys) with explicit
memory zeroing.

```go
ss := secure.NewString("password")
defer ss.Clear() // zeros memory, returns to pool

value := ss.String()         // allocating copy
value = ss.StringUnsafe()    // zero-copy (invalid after Clear)
data := ss.Bytes()           // allocating copy
defer secure.ZeroBytes(data)
```

### Properties

- Memory zeroing: `Clear()` overwrites stored data with zeros.
- Small-string optimization: strings ≤64 bytes stored inline (no heap allocation).
- Object pooling: instances from `NewString` are recycled via `sync.Pool`.
- GC safety: a runtime cleanup zeros heap data if the instance is collected
  without `Clear`.
- Redacted debug output: `GoString()` returns `secure.String{<redacted>}`.

### Constant-time comparison

```go
if stored.Equal(provided) {
    // timing-safe comparison via crypto/subtle.ConstantTimeCompare
}
```

### Standalone helpers

| Function | Description |
|----------|-------------|
| `Equal(a, b)` | Constant-time string equality |
| `PrefixMatch(s, prefix)` | Constant-time case-insensitive prefix check |
| `SubstringMatch(s, substr)` | Constant-time case-insensitive substring search |
| `strings.SubstringMatch(s, substr)` | Case-insensitive substring (NOT constant-time; parent package) |
| `ZeroBytes(data)` | Overwrite byte slice with zeros |
| `ZeroString(s)` | Best-effort zeroing of string backing memory (silent no-op for literals / read-only memory; never panics) |

---

## Object pools

### `strings.Builder` pool

```go
b := strings.GetStringBuilder()
defer strings.PutStringBuilder(b)
b.Grow(128)
b.WriteString("hello")
result := b.String()
```

Or use the convenience wrapper:

```go
result := strings.BuildString(func(b *strings.Builder) {
    b.WriteString("hello")
    b.WriteString(" world")
})
```

### String slice pool

```go
s := strings.GetStringSlice()                    // capacity ≥ 64
s = strings.GetStringSliceWithCapacity(256)       // capacity ≥ 256
defer strings.PutStringSlice(s)
```

Element references are cleared on return to allow GC of referenced strings.

---

## See also

