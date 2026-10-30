# signals

```go
import "github.com/altessa-s/go-atlas-core/runtime/signals"
```

Package `signals` provides OS signal handling with priority-based execution and graceful shutdown. Fully thread-safe: registration, start/stop, and
signal processing can all be called concurrently without external synchronization.

## Quick start

```go
handler := signals.New(
    signals.WithSignals(syscall.SIGTERM, syscall.SIGINT),
    signals.WithWorkerPoolSize(20),
    signals.WithHandlerTimeout(10 * time.Second),
    signals.WithShutdownTimeout(30 * time.Second),
    signals.WithExecutionMode(signals.ParallelMode),
)

handler.AddHandlerWithPriority(func(ctx context.Context, sig os.Signal) error {
    return flushMetrics(ctx)
}, signals.PriorityHigh, syscall.SIGTERM)

stop := make(chan struct{}, 1)
handler.AddHandler(func(ctx context.Context, sig os.Signal) error {
    select { // notify the coordinator and return; never call Shutdown from a handler
    case stop <- struct{}{}:
    default:
    }
    return closeConnections(ctx)
}, syscall.SIGTERM, syscall.SIGINT)

handler.Start()
<-stop
if err := handler.Shutdown(ctx); err != nil { // waits for in-flight handlers, runs shutdown hooks
    logger.Error("shutdown", slog.Any("error", err))
}
```

## Options

| Option                    | Default                    | Description                                                                            |
|---------------------------|----------------------------|----------------------------------------------------------------------------------------|
| `WithSignals`             | —                          | OS signals to listen for                                                               |
| `WithWorkerPoolSize`      | 10                         | Max concurrent handler goroutines                                                      |
| `WithShutdownTimeout`     | 30s                        | Graceful shutdown deadline                                                             |
| `WithHandlerTimeout`      | 5s                         | Per-handler execution timeout                                                          |
| `WithExecutionMode`       | Sequential                 | `SequentialMode` or `ParallelMode`                                                     |
| `WithErrorHandler`        | —                          | Callback for handler errors/panics                                                     |
| `WithSignalChannelBuffer` | 1                          | Buffer size of the OS signal channel                                                   |
| `WithShutdownHooks`       | `runtime.RunShutdownHooks` | Hooks run at the end of `Shutdown`; pass `(*runtime.HookGroup).Shutdown` to scope them |
| `WithClock`               | `clock.Real()`             | Clock driving the parallel-mode shutdown grace period                                  |

## Signal methods

`New(opts...)` returns a `*Signal`; registration and start methods return the same `*Signal` for chaining.

| Method                                          | Description                                                       |
|-------------------------------------------------|-------------------------------------------------------------------|
| `AddHandler(h, sigs...)`                        | Register a `ContextHandler` at `PriorityNormal` for given signals |
| `AddHandlerWithPriority(h, p, sigs...)`         | Register a handler with an explicit `Priority`                    |
| `AddBroadcastHandler(h)`                        | Register a handler for every received signal                      |
| `AddBroadcastHandlerWithPriority(h, p)`         | Broadcast handler with an explicit `Priority`                     |
| `Start()`                                       | Begin listening for the configured signals (idempotent)           |
| `Shutdown(ctx)` / `Stop()`                      | Stop listening and wait for in-flight handlers (idempotent)       |
| `Wait()`                                        | Block until the handler has stopped                               |

Multiple handlers can be registered for the same signal; a nil handler is silently ignored. Broadcast handlers are merged with signal-specific
handlers and sorted by priority together before each dispatch, so a `PriorityHighest` broadcast handler still runs before a `PriorityNormal`
signal-specific one.

### Execution modes

- **`SequentialMode`** (default) — handlers run one at a time, in priority order, within each signal's dispatch. A higher-priority handler must
  complete before a lower-priority one starts — unless it has a handler timeout and exceeds it, in which case the next handler starts while the
  timed-out one may still be running (it should honor `ctx.Done()`).
- **`ParallelMode`** — independent handlers run concurrently, up to the worker pool size. Priority determines startup order but not completion
  order.

### Worker pool

A single process-wide worker pool (`WithWorkerPoolSize`) limits concurrent handler goroutines across every signal. When all slots are occupied:

- **In the signal listener** — the new signal is dropped and the error handler is called with `"worker pool is full, signal handler dropped"`.
- **In `ParallelMode` dispatch** — the handler executes synchronously in the current goroutine instead of spawning one, to avoid goroutine
  explosion.

On a `Shutdown` timeout the handler context is canceled and the shutdown hooks still run, but handlers that ignore cancellation keep running: their
goroutines are not reclaimed, and a later `Wait` blocks until they exit. `Shutdown` rejects a nil `ctx` with an error.

## Priority levels

| Constant          | Value | Use case                          |
|-------------------|-------|-----------------------------------|
| `PriorityHighest` | 100   | Critical system operations        |
| `PriorityHigh`    | 75    | Important business logic          |
| `PriorityNormal`  | 50    | Default                           |
| `PriorityLow`     | 25    | Background tasks                  |
| `PriorityLowest`  | 1     | Logging, metrics                  |

Custom `Priority` values outside 1-100 are supported: they sort relative to the predefined levels, and handlers sharing a priority run in
registration order.

## Timeouts

### Handler timeout

Each handler receives a context derived from the global context with a deadline set to `WithHandlerTimeout`. If the handler does not complete in
time, a `*TimeoutError` is reported to the error handler; the handler's goroutine is not forcibly killed — it should respect `ctx.Done()`. When
`handlerTimeout` is zero or negative, the handler receives the global context without an additional deadline — a fast path with no extra goroutine.

### Shutdown timeout

`Shutdown(ctx)` applies `WithShutdownTimeout` via `corecontext.ApplyTimeout`: if `ctx` already carries any deadline — tighter or looser — it is kept
unchanged and the shutdown timeout is not applied. If in-flight handlers do not complete within the resulting window, shutdown returns a wrapped
context error.

## Error types

| Type            | Description                                          |
|-----------------|------------------------------------------------------|
| `*TimeoutError` | Handler exceeded its timeout; check with `IsTimeout` |
| `*PanicError`   | Handler panicked; recovered value in `Panic` field   |

Register a custom error handler via `WithErrorHandler` to observe failures:

```go
signals.New(
    signals.WithErrorHandler(func(sig os.Signal, err error) {
        if signals.IsTimeout(err) {
            logger.Warn("handler timed out", slog.Any("signal", sig))
            return
        }
        logger.Error("handler failed", slog.Any("signal", sig), slog.Any("error", err))
    }),
    signals.WithSignals(syscall.SIGTERM),
)
```

Panics within the error handler itself are recovered and silently discarded, to prevent a broken error handler from cascading into the signal
dispatch path.

## Lifecycle

- **`Start`** begins listening for OS signals via `signal.Notify` and dispatching to handlers. Idempotent.
- **`Wait`** blocks until the signal listener stops, which happens only after `Shutdown` (or `Stop`) is called from elsewhere. It does not itself
  initiate shutdown — use it to keep the main goroutine alive while another goroutine drives shutdown.
- **`Shutdown(ctx)`** runs a graceful sequence: (1) stops receiving new OS signals (`signal.Stop`); (2) signals the background listener to exit;
  (3) waits for all in-flight handlers to complete, or the deadline to expire; (4) runs the configured shutdown hooks (`WithShutdownHooks`,
  `runtime.RunShutdownHooks` by default); (5) cancels the global handler context. If `ctx` expires before step 3 finishes, the handler context is
  canceled first so in-flight handlers unwind, then shutdown hooks are still attempted on a best-effort basis with `context.Background()`.
- **`Stop()`** is a convenience wrapper that calls `Shutdown(context.Background())`.

Both `Shutdown` and `Stop` are idempotent — calls after the first return nil immediately.

## Integration with shutdown hooks

`Shutdown` runs the configured shutdown hooks only after every in-flight signal handler completes — receiving an OS signal alone does not run
them. Because of that ordering, **never call `Shutdown` synchronously from inside a signal handler**: `Shutdown` waits for in-flight handlers,
including the one calling it, so it will block until its own shutdown timeout and then report a timeout error. Likewise, don't call `Wait` first
and expect it to trigger shutdown — `Wait` only returns after `Shutdown` has already stopped the listener.

Instead, have the handler notify a coordinator (e.g. close or send on a channel) and return; the coordinator then calls `Shutdown`, as in
[Quick start](#quick-start) above:

```go
// At startup — registered on runtime's process-wide shutdown hooks:
runtime.OnShutdown(func(ctx context.Context) error {
    return db.Close()
})

// Shutdown (or Stop) runs the configured shutdown hooks after in-flight handlers finish:
handler := signals.New(signals.WithSignals(syscall.SIGTERM))
stop := make(chan struct{}, 1)
handler.AddHandler(func(ctx context.Context, sig os.Signal) error {
    select {
    case stop <- struct{}{}:
    default:
    }
    return nil
}, syscall.SIGTERM)
handler.Start()

<-stop
if err := handler.Shutdown(ctx); err != nil {
    logger.Error("shutdown error", slog.Any("error", err))
}
```

See [../README.md](../README.md) for the process-wide shutdown hook ordering and guarantees, and `WithShutdownHooks` above to scope them to a
`runtime.HookGroup` instead.

## Performance

- **Dispatch sorting** — at or below `LargeHandlerCountThreshold` (50) total handlers for the signal, the dispatcher sorts by priority with an
  in-place insertion sort for ≤32 handlers (O(n²) but zero-allocation and cache-friendly, which wins at small N) or a stable counting sort for more
  (O(n+k), where k is the observed priority range), falling back to insertion sort when the range is wider than 1024 distinct levels or more than
  8x sparser than the handler count. Above the 50-handler threshold, sorting is skipped entirely in favor of a bucket-based priority queue with
  O(1) insertion — one fixed bucket per predefined priority level plus one bucket per distinct custom priority, merged in descending order at
  dispatch time.
- **Zero-timeout fast path** — handlers without a timeout execute synchronously in the caller's goroutine, with no extra goroutine or channel
  overhead.
- **Worker pool** — a channel-based semaphore prevents goroutine explosion under heavy signal load; see [Worker pool](#worker-pool) above.

### Tuning constants

| Constant                      | Value | Purpose                                              |
|--------------------------------|-------|-------------------------------------------------------|
| `LargeHandlerCountThreshold`  | 50    | Above this handler count, the priority queue is used |
| `HandlerSliceInitialCapacity` | 32    | Pre-allocated handler slice capacity                 |
| `PriorityBucketCount`         | 5     | Number of predefined priority queue buckets          |
| `ResponsiveTimeoutDuration`   | 50ms  | Grace period for parallel handlers after shutdown    |
