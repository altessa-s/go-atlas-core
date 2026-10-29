# go-atlas-core

```go
import "github.com/altessa-s/go-atlas-core/<package>"
```

Foundation packages of the [go-atlas](https://github.com/altessa-s/go-atlas) toolkit, usable on their own. Packages use only the Go standard
library and each other — with narrow, package-scoped exceptions: `io/spoolbudget` uses `golang.org/x/sync/semaphore`, `collections/lru` uses
`hashicorp/golang-lru`, `scheduler` uses `robfig/cron`, and the Linux sandboxing packages (`runtime/capabilities`, `landlock`, `nonewprivs`,
`rlimits`, `seccomp`) use `golang.org/x/sys/unix`. Each package can be imported independently without pulling in the rest of the module.

## Packages

| Package                      | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| [collections](./collections) | Generic maps, slices and LRU utilities, iterators, pools                    |
| [context](./context)         | Context timeout helpers                                                     |
| [encoding](./encoding)       | Hashing, serialization, composite keys and entity-tags (ETags)              |
| [errors](./errors)           | Error wrapping, construction, and classification                            |
| [id](./id)                   | Service identity                                                            |
| [io](./io)                   | Buffer pools, limited readers, file-system helpers, spooling, WAL           |
| [retry](./retry)             | Stdlib-only retry loop with pluggable delay policies                        |
| [runtime](./runtime)         | Shutdown hooks, concurrency, singleflight, signals, panics, sandboxing      |
| [scheduler](./scheduler)     | Task scheduling interface and priority model                                |
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
| `make generate` | Install `optgen` from `devtools` and run go generate |
| `make bench`    | Run all benchmarks                                   |

`devtools/go.mod` resolves `optgen` from a sibling go-atlas checkout (`replace` to `../../go-atlas.cel`) until go-atlas is tagged.
