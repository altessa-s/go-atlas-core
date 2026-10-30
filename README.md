# go-atlas-core

```go
import "github.com/altessa-s/go-atlas-core/<package>"
```

Foundation packages for Go services and libraries: error wrapping, retries, bounded concurrency, singleflight, shutdown hooks, signals, panics,
Linux sandboxing, collections, encoding, I/O (spool, WAL), text, time and type helpers. Packages use only the Go standard
library and each other — with narrow, package-scoped exceptions: `io/spoolbudget` uses `golang.org/x/sync/semaphore`, `collections/lru` uses
`hashicorp/golang-lru`, and the Linux sandboxing packages (`runtime/capabilities`, `landlock`, `nonewprivs`,
`rlimits`, `seccomp`) use `golang.org/x/sys/unix`. `text/charset` uses `golang.org/x/text` for charset lookup and decoding; Linux seccomp tests
use `golang.org/x/net/bpf` to interpret filters. Each package can be imported independently without pulling in the rest of the module.

## Packages

| Package                      | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| [collections](./collections) | Generic maps, slices and LRU utilities, iterators, pools                    |
| [context](./context)         | Context timeout helpers                                                     |
| [encoding](./encoding)       | Hashing, serialization, composite keys and entity-tags (ETags)              |
| [errors](./errors)           | Error wrapping, construction, classification; network checks (`network`)    |
| [id](./id)                   | Service identity                                                            |
| [io](./io)                   | Buffer pools, limited readers, file-system helpers, spooling, WAL           |
| [retry](./retry)             | Stdlib-only retry loop with pluggable delay policies                        |
| [runtime](./runtime)         | Shutdown hooks, concurrency, singleflight, signals, panics, sandboxing      |
| [text](./text)               | String manipulation, interning, secure storage, charsets                    |
| [time](./time)               | Time formatting, calendar dates (`time/civil`), clocks (`time/clock`)       |
| [types](./types)             | Bits, constraints, nil checks, optional, pointers, redacted strings, result |

## Design principles

- **Minimal dependencies** — the Go standard library, plus the package-scoped exceptions listed above.
- **Thread-safe by default** — all exported functions and types are safe for concurrent use unless documented otherwise.
- **Generic-first** — uses Go generics (`constraints.Integer`, `constraints.Primitive`, etc.) to minimize type-specific boilerplate.
- **Allocation-aware** — `sync.Pool` wrappers, zero-copy conversions, and iterator-based APIs to reduce GC pressure.

## Development

| Command         | Description                                          |
|-----------------|------------------------------------------------------|
| `make test`     | Run all tests                                        |
| `make lint`     | golangci-lint with the repo config (`.golangci.yml`) |
| `make generate` | Install `optgen` and run go generate                 |
| `make bench`    | Run all benchmarks                                   |

`make generate` installs `optgen` from a sibling [go-optgen](https://github.com/altessa-s/go-optgen) checkout (`OPTGEN_DIR`, default `../go-optgen`).
