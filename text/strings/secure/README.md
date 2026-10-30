# secure

```go
import "github.com/altessa-s/go-atlas-core/text/strings/secure"
```

Package `secure` handles sensitive string data: zeroing-on-clear storage, best-effort in-place zeroing, and timing-safe comparisons. General string
helpers live in the parent [strings](../README.md) package; string deduplication lives in the sibling [intern](../intern/README.md) package.

Go strings are immutable and the garbage collector decides when their backing memory is reclaimed, so a plaintext secret held in an ordinary
`string` can linger in the heap long after the variable goes out of scope, and `==` comparison on secrets is vulnerable to timing side-channels
that leak how many leading bytes matched. `String` gives a secret storage that the caller can zero deterministically on `Clear()`, and the
comparison helpers run in constant time so neither presence nor position of a match is observable through timing.

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

`ZeroBytes` uses a manual byte loop for slices at or below 32 bytes and the `clear()` builtin above that threshold, whichever is faster at that
size. `ZeroString` writes through an unsafe pointer and recovers from the fault that read-only memory (string literals, compiler-folded constants)
raises on write, so it never panics — but that also means it silently does nothing to such strings. An interned string (see
[intern](../intern/README.md)) is the same case: it may live in read-only storage or share its backing with every other holder of that value, so a
successful write there would corrupt all of them. Never rely on `ZeroString` to erase a secret that might be interned; keep secrets in `String`,
which always owns private, writable storage. `ZeroString` is most useful as defense-in-depth for heap-allocated strings obtained from I/O (user
input, network reads) where `String` was not used from the start.

## Comparison

| Function         | Description                                      |
|------------------|--------------------------------------------------|
| `Equal`          | Constant-time string equality                    |
| `PrefixMatch`    | Constant-time, case-insensitive prefix check     |
| `SubstringMatch` | Constant-time, case-insensitive substring search |

`Equal` converts both operands with the parent package's zero-copy `ToBytesUnsafe`, so it allocates nothing. `PrefixMatch` pads a short `s` with
null bytes before comparing so the result does not leak whether `s` was too short through timing, but it and `SubstringMatch` both lowercase their
inputs with `strings.ToLower` first, which does allocate. `SubstringMatch` checks every starting position in `s` with
`crypto/subtle.ConstantTimeCompare`, costing O(len(s) × len(substr)); reach for it only when a timing leak on substring search is a genuine
concern, otherwise prefer the parent package's non-constant-time `strings.SubstringMatch`.

## Usage

```go
ss := secure.NewString("password")
defer ss.Clear()

if secure.PrefixMatch(header, "bearer ") {
    // ...
}
```
