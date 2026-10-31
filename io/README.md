# io

```go
import "github.com/altessa-s/go-atlas-core/io"
```

Package `io` provides I/O utilities including buffer pooling, size-limited readers, and test helpers for simulating read errors.

## Buffer pool

`GetBuffer` / `PutBuffer` wrap a `sync.Pool` of `*bytes.Buffer`. Buffers exceeding 64 KB are discarded on return to prevent unbounded memory retention
in long-running services. Typical usage: borrow via `GetBuffer`, write, then `PutBuffer`.

## LimitedReadCloser

`NewLimitedReadCloser(rc, limit)` wraps an `io.ReadCloser` with a byte-count limit. It never delivers more than `limit` bytes: when the source holds
more, `Read` returns the bytes up to the limit with `ErrReadLimitExceeded` instead of silently truncating, and every later `Read` returns the error
again without data. A source of exactly `limit` bytes reads to `io.EOF` without error. To detect the overflow it reads one byte past the limit from the
source; that byte is never handed to the caller. Safe for concurrent reads — a mutex serializes each `Read` with the running byte total, so the limit
holds across goroutines; `Close` delegates to the underlying closer. An underlying read error remains available through `errors.Is` when the same read
exceeds the limit.

## ReadAllLimited

`ReadAllLimited(r, n)` is the one-shot form: it reads `r` to EOF like `io.ReadAll` but reads at most `n+1` bytes, and when more than `n` are available it
returns the first `n` bytes with an error wrapping `ErrReadLimitExceeded`. Read errors are returned unchanged with the bytes read before them; a
negative `n` is treated as 0; `r` is not closed. Use it for bounded response bodies instead of `io.ReadAll(io.LimitReader(r, n+1))` plus a length
check.

## ErrorReader

`NewErrorReader(err)` returns an `io.Reader` that always fails with the given error. Useful for tests that need to simulate I/O failures without hitting
the file system.

## RangeReadSeeker

Adapts a `RangeOpener` — any "open a ranged read at offset" function (S3 `GetObject` with a `Range` header, an HTTP Range request) — to
`io.ReadSeekCloser` (`NewRangeReadSeeker(ctx, size, open)`) for consumers that need random access over a remote object without downloading it up front.
Reads lazily open a ranged request at the current position; `Seek` only moves the position and forces the next `Read` to reopen. A body shorter than the
requested range surfaces as `io.ErrUnexpectedEOF`; `Seek` rejects bad input with `ErrInvalidWhence` or `ErrNegativePosition`. Not safe for concurrent
use; the caller must `Close` it to release the open body.

## Spool

The [`spool`](spool) subpackage materializes an `io.Reader` exactly once into a local, rewindable `io.ReadSeekCloser` — in memory for small content,
in a temp file once it exceeds the memory threshold — so consumers can re-read it from offset 0 without re-touching the (possibly remote) origin.
Options: `spool.WithMemThreshold` overrides the memory/file cutoff (`spool.DefaultMemThreshold`, 4 MiB); `spool.WithMaxBytes(n)` caps the size and
returns `spool.ErrTooLarge` past it (`n <= 0` is unlimited); `spool.WithTee(w)` mirrors every byte into `w` during the single pass so a digest (e.g.
SHA-256) can be computed without a second read; `spool.WithTempDir` picks the spill directory. `Size` reports the materialized length. The source is
never closed (caller owns it); `Close` removes a spilled temp file and is idempotent. Not safe for concurrent use.

## Subpackages

| Package                      | Description                                                                |
|------------------------------|----------------------------------------------------------------------------|
| [files](files)             | File/directory existence checks, multi-path search                         |
| [spool](spool)             | Materialize a reader into a rewindable in-memory/temp-file backing store    |
| [spoolbudget](spoolbudget) | Process-wide disk ceiling charged across concurrent `Spool` reservations   |
| [wal](wal)                 | Segmented append-only write-ahead log with CRC32, fsync policy, recovery   |
