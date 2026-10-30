# strings

```go
import "github.com/altessa-s/go-atlas-core/text/strings"
```

Package `strings` provides string manipulation, conversion, case transformation, truncation, and pooling utilities. All utility functions are pure,
allocation-aware, and safe for concurrent use from multiple goroutines. Secret handling lives in [secure](secure/README.md) and string interning in
[intern](intern/README.md).

## Validation and conversion

| Function                    | Description                                               |
|-----------------------------|-----------------------------------------------------------|
| `IsEmpty`                   | True if string/`*string` is empty or whitespace (generic) |
| `IsEmptyOrWhitespace`       | Same for plain strings (allocation-free)                  |
| `ToPtr`                     | String to `*string`, nil when empty or whitespace-only     |
| `FromPtr`                   | `*string` to string, `""` when nil                        |
| `To[T]`                     | Parse string to any numeric type, zero value on parse error |
| `ToInt64`, `ToFloat64`, ... | Pre-instantiated numeric converters                       |

`IsEmpty` checks emptiness via `strings.TrimSpace`, recognizing every Unicode whitespace code point; `IsEmptyOrWhitespace` is the allocation-free
alternative for plain strings — it iterates runes directly and returns on the first non-space rune instead of trimming both ends.

## Case and trimming

| Function                            | Description                            |
|-------------------------------------|----------------------------------------|
| `ToSnakeCase`                       | `"HelloWorld"` -> `"hello_world"`      |
| `ToCamelCase`                       | `"hello_world"` -> `"helloWorld"`      |
| `ToScreamingSnakeCase`              | `"HelloWorld"` -> `"HELLO_WORLD"`      |
| `ScreamingSnakeToCamelCase`         | `"HELLO_WORLD"` -> `"helloWorld"`      |
| `IsLowercase` / `IsUppercase`       | Check letter case (Unicode-aware)      |
| `IsTrimmed`                         | True if no leading/trailing whitespace |
| `TrimPrefixFast` / `TrimSuffixFast` | Zero-allocation prefix/suffix removal  |
| `IsLowercaseUnsafe` / `IsUppercaseUnsafe` / `IsTrimmedUnsafe` | ASCII fast-path variants (Unicode fallback) |
| `IsASCIISpace`                      | ASCII whitespace byte check            |

All four case-conversion functions use an ASCII fast path with a Unicode (rune-based) fallback for non-ASCII input; the camelCase functions copy
invalid UTF-8 bytes through unchanged instead of substituting U+FFFD. `ToCamelCase` lowercases only the first letter and leaves every other letter
that does not start a word untouched, so `"SCREAMING_SNAKE"` becomes `"sCREAMINGSNAKE"`; `ScreamingSnakeToCamelCase` additionally lowercases the rest
of each word (`"REAL_IP"` becomes `"realIp"`). Both treat underscores and Unicode whitespace as word delimiters.

## Truncation

| Function        | Description                                                                   |
|-----------------|-------------------------------------------------------------------------------|
| `TruncateRunes` | Cut to at most `n` runes, ending with `suffix` when cut                       |
| `TruncateBytes` | Cut to at most `n` bytes on a UTF-8 boundary, ending with `suffix` when cut   |

The suffix counts toward the limit: the result, suffix included, never exceeds `n` runes (bytes). When `n` is smaller than the suffix, the result is
the prefix that fits, without the suffix. A string within the limit is returned unchanged, `n <= 0` yields `""`, and whitespace is never trimmed —
callers trim first when they need to. `TruncateRunes("привет мир", 5, "…")` is `"прив…"`; `TruncateBytes("привет", 5, "")` is `"пр"`.

## Join and Split

Configurable via `JoinOptions`, `SplitOptions`, and `ContainsOptions` structs. All operations support case sensitivity and empty-value skipping.

| Function   | Description                                                  |
|------------|--------------------------------------------------------------|
| `Join`     | Concatenate with separator, prefix/suffix, skip-empty        |
| `Split`    | Split with case sensitivity, max splits, trim, skip-empty    |
| `SplitSeq` | Lazy `iter.Seq` variant of `Split` (same parts, no slice)    |
| `Contains` | Search with case, whole-word, and count options              |

Case-insensitive splitting matches the separator under Unicode simple case folding (the rule used by `strings.EqualFold`), so `"ſ"` matches `"s"`,
the Kelvin sign matches `"k"`, and `"Σ"`, `"σ"`, `"ς"` match each other. Pure ASCII input and separator use a fast lowercase scan; anything else uses
a cached case-insensitive regex bounded at 256 entries. A separator that is not valid UTF-8 is matched exactly. `Contains` returns as soon as the
first match is confirmed when `ContainsOptions.Count` is false; counting all occurrences and their positions costs an extra pass.

```go
parts := strings.Split("a,,b", strings.SplitOptions{Separator: ",", SkipEmpty: true}) // ["a", "b"]
joined := strings.Join([]string{"a", "", "b"}, strings.JoinOptions{Separator: ",", SkipEmpty: true}) // "a,b"

r := strings.Contains("Hello World", "hello", strings.ContainsOptions{CaseSensitive: false}) // r.Found == true
```

## Concatenation

| Function       | Description                                  |
|----------------|----------------------------------------------|
| `Concat`       | Pooled `strings.Builder`, single allocation  |
| `ConcatUnsafe` | Single `[]byte` allocation, zero-copy result |

Both return the original string unchanged for single-element input (no allocation) and the empty string for zero elements.

## Comparison

| Function         | Description                                                                   |
|------------------|-------------------------------------------------------------------------------|
| `SubstringMatch` | Case-insensitive substring (NOT constant-time; see [secure](secure/README.md) for that) |

## Pooling

| Function              | Description                                      |
|-----------------------|--------------------------------------------------|
| `GetStringSlice`      | Borrow a `[]string` from the pool                |
| `PutStringSlice`      | Return a `[]string` to the pool                  |
| `GetStringSliceWithCapacity` | Borrow a `[]string` with a capacity hint   |
| `GetStringBuilder`    | Borrow a `strings.Builder` from the pool         |
| `PutStringBuilder`    | Return a `strings.Builder` to the pool           |
| `BuildString`         | Borrow, build, return in one call                |

`GetStringSlice` guarantees a capacity of at least 64; `GetStringSliceWithCapacity` takes a capacity hint and allocates fresh only when the pooled
slice is too small. `PutStringSlice` clears element references before returning the backing array to the pool, so strings referenced only by a
pooled slice remain eligible for GC instead of being pinned until the slot is reused.

```go
result := strings.BuildString(func(b *strings.Builder) {
    b.WriteString("hello")
    b.WriteString(" world")
})
```

## Zero-copy (unsafe)

| Function             | Description                                        |
|----------------------|----------------------------------------------------|
| `ToBytesUnsafe`      | String to `[]byte` without copy (**read-only**)    |
| `FromBytesUnsafe`    | `[]byte` to string without copy (**don't modify**) |
| `StringEqualsUnsafe` | Pointer-compare interned strings, then fallback    |

Both unsafe conversions use `unsafe.StringData`/`unsafe.SliceData` (Go 1.20+) and return the zero value (`nil` / `""`) for empty input rather than a
slice or string header pointing at invalid memory. `StringEqualsUnsafe` rejects differing lengths immediately, then compares data pointers in O(1)
(common for interned strings) before falling back to `==`; unlike `strings.EqualFold`, it is always case-sensitive.
