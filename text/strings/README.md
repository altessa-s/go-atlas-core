# strings

```go
import "github.com/altessa-s/go-atlas-core/text/strings"
```

Package `strings` provides string manipulation, conversion, case transformation, truncation, and pooling utilities. All utility functions are pure,
allocation-aware, and safe for concurrent use from multiple goroutines. Secret handling lives in [secure](secure) and string interning in
[intern](intern).

## Validation and conversion

| Function                    | Description                                               |
|-----------------------------|-----------------------------------------------------------|
| `IsEmpty`                   | True if string/`*string` is empty or whitespace (generic) |
| `IsEmptyOrWhitespace`       | Same for plain strings (allocation-free)                  |
| `ToPtr`                     | String to `*string`, nil when empty                       |
| `FromPtr`                   | `*string` to string, `""` when nil                        |
| `To[T]`                     | Parse string to any numeric type                          |
| `ToInt64`, `ToFloat64`, ... | Pre-instantiated numeric converters                       |

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

## Concatenation

| Function       | Description                                  |
|----------------|----------------------------------------------|
| `Concat`       | Pooled `strings.Builder`, single allocation  |
| `ConcatUnsafe` | Single `[]byte` allocation, zero-copy result |

## Comparison

| Function         | Description                                                                   |
|------------------|-------------------------------------------------------------------------------|
| `SubstringMatch` | Case-insensitive substring (NOT constant-time; see [secure](secure) for that) |

## Pooling

| Function              | Description                                      |
|-----------------------|--------------------------------------------------|
| `GetStringSlice`      | Borrow a `[]string` from the pool                |
| `PutStringSlice`      | Return a `[]string` to the pool                  |
| `GetStringSliceWithCapacity` | Borrow a `[]string` with a capacity hint   |
| `GetStringBuilder`    | Borrow a `strings.Builder` from the pool         |
| `PutStringBuilder`    | Return a `strings.Builder` to the pool           |
| `BuildString`         | Borrow, build, return in one call                |

## Zero-copy (unsafe)

| Function             | Description                                        |
|----------------------|----------------------------------------------------|
| `ToBytesUnsafe`      | String to `[]byte` without copy (**read-only**)    |
| `FromBytesUnsafe`    | `[]byte` to string without copy (**don't modify**) |
| `StringEqualsUnsafe` | Pointer-compare interned strings, then fallback    |
