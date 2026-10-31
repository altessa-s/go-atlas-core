# panics

```go
import "github.com/altessa-s/go-atlas-core/runtime/panics"
```

Package `panics` provides panic recovery with pluggable handlers and runtime assertion helpers. All configuration uses atomic operations and is
thread-safe — handlers can be registered and modified concurrently without external locks.

## Quick start

```go
go func() {
    defer panics.Handle(ctx)
    riskyWork()
}()
```

Assertion helpers convert contract violations into panics with clear messages:

```go
panics.MustNonNil(cfg, "config is required")
panics.MustNonZero(timeout, "timeout must be set")
panics.Must(port > 0, "port must be positive")

val := panics.MustResult(parseConfig(path)) // panics on error, returns value
```

Register a handler that runs on every recovered panic across the application:

```go
panics.AddGlobalPanicHandler(func(ctx context.Context, r any) {
    metrics.IncrCounter("panics_total", 1)
})
```

## Recovery

| Function         | Description                                                          |
|------------------|----------------------------------------------------------------------|
| `Handle`         | Recover and run global + per-call handlers                           |
| `HandleWithOpts` | Same with per-call options (e.g., override re-panic)                 |
| `NewHandleOpts`  | Build `*HandleOpts`; chain `SetReallyPanic` / `SetShutdown` per call |

Both must be deferred directly. Handlers are `PanicHandler` funcs (`func(ctx context.Context, r any)`); a re-thrown panic first runs the shutdown
function — `runtime.RunShutdownHooks` by default, or the one set with `HandleOpts.SetShutdown` (e.g. a `runtime.HookGroup`'s `Shutdown`).

## Guarded channel sends

| Function             | Description                                                                |
|----------------------|----------------------------------------------------------------------------|
| `TrySend`            | Blocking send, canceled by `ctx`; recovers the send-on-closed panic        |
| `TrySendNonBlocking` | Select-with-default send; recovers the send-on-closed panic                |

Both report `(sent, closed bool)`. Needing them is a channel-ownership smell — the sender should own the close — but they contain the crash when that
ownership is shared or inverted.

## Assertions

| Function          | Panics when                                      |
|-------------------|--------------------------------------------------|
| `Must`            | `ok` is `false`                                  |
| `MustNonNil`      | value is `nil`                                   |
| `MustNonZero`     | value is `nil` or zero value                     |
| `MustError`       | `err` is non-nil                                 |
| `MustResult`      | `err` is non-nil; otherwise returns `T`          |
| `InvalidArgument` | condition is `true` (wraps `ErrInvalidArgument`) |

`Must`, `MustNonNil` and `MustNonZero` take `msg` as a string, a `*string` or a type defined over either, and panic with its text as a `string`. A nil
pointer is never dereferenced: the panic value is its `%#v` form, such as `(*string)(nil)`.

## Configuration

| Function                 | Description                                         |
|--------------------------|-----------------------------------------------------|
| `SetReallyPanic`         | Re-panic after handlers (default: `false`)          |
| `SetLogger`              | Set `slog.Logger` for the built-in handler          |
| `SetLoggerFromContext`   | Extract logger from context in the built-in handler |
| `AddGlobalPanicHandler`  | Append a handler to the global list                 |
| `SetGlobalPanicHandlers` | Replace all global handlers                         |
