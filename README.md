# go-atlas-core

[![CI](https://github.com/altessa-s/go-atlas-core/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/altessa-s/go-atlas-core/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/altessa-s/go-atlas-core.svg)](https://pkg.go.dev/github.com/altessa-s/go-atlas-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/altessa-s/go-atlas-core)](https://goreportcard.com/report/github.com/altessa-s/go-atlas-core)
[![Release](https://img.shields.io/github/v/release/altessa-s/go-atlas-core)](https://github.com/altessa-s/go-atlas-core/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/altessa-s/go-atlas-core)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

```go
import "github.com/altessa-s/go-atlas-core/<package>"
```

Foundation packages for Go services and libraries: error wrapping, retries, bounded concurrency, singleflight, shutdown hooks, signals, panics,
Linux sandboxing, collections, encoding, I/O (spool, WAL), text, time and type helpers.

Each package can be imported independently without pulling in the rest of the module.

## Requirements

Go 1.27 or later (`go 1.27.0` in `go.mod`). The module uses Go 1.27 APIs — `encoding/json/v2`, `testing/synctest.Sleep`, the `goroutineleak`
profile — so earlier toolchains cannot build it.

## Packages

| Package                      | Description                                                                 |
|------------------------------|-----------------------------------------------------------------------------|
| [collections](./collections) | Generic maps and slices utilities, iterators, pools                         |
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

`make generate` installs [`optgen`](https://github.com/altessa-s/go-optgen) at the version pinned in `devtools/go.mod`.
