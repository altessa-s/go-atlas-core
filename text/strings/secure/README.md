# secure

```go
import "github.com/altessa-s/go-atlas-core/text/strings/secure"
```

Package `secure` handles sensitive string data: zeroing-on-clear storage, best-effort in-place zeroing, and timing-safe comparisons. General string
helpers live in the parent [strings](..) package.

## String

Tamper-resistant storage for sensitive data (passwords, tokens, keys). Zeroes memory on `Clear()`, uses inline storage for strings up to 64 bytes
(`DefaultSmallStringOptimizationThreshold`), and returns instances to a pool to reduce allocations in high-throughput code paths. `GoString()`
returns `GoStringRedacted` (or `GoStringEmpty`), so `%#v` never prints the secret.

| Function / Method       | Description                       |
|-------------------------|-----------------------------------|
| `NewString`             | Construct from a string (pooled)  |
| `String`                | Safe copy of stored data          |
| `StringUnsafe`          | Zero-copy (invalid after `Clear`) |
| `Bytes` / `BytesUnsafe` | Byte slice accessors              |
| `Equal`                 | Constant-time comparison          |
| `Len` / `IsEmpty`       | Length and emptiness checks       |
| `Clear`                 | Zero memory and return to pool    |

## Zeroing

| Function     | Description                                                                       |
|--------------|-----------------------------------------------------------------------------------|
| `ZeroBytes`  | Overwrite a byte slice with zeros                                                 |
| `ZeroString` | Best-effort zeroing of a string's backing memory (silent no-op on read-only data) |

## Comparison

| Function         | Description                                      |
|------------------|--------------------------------------------------|
| `Equal`          | Constant-time string equality                    |
| `PrefixMatch`    | Constant-time, case-insensitive prefix check     |
| `SubstringMatch` | Constant-time, case-insensitive substring search |

## Usage

```go
ss := secure.NewString("password")
defer ss.Clear()

if secure.PrefixMatch(header, "bearer ") {
    // ...
}
```
