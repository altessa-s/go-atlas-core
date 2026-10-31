# singleflight

```go
import "github.com/altessa-s/go-atlas-core/runtime/singleflight"
```

Deduplicates concurrent computations using Go key equality. Each caller can cancel its wait independently of the shared operation.

## Options

| Option              | Default                       | Meaning                                                          |
|---------------------|-------------------------------|------------------------------------------------------------------|
| `WithLifecycle`     | `nil`                         | Parent and last-waiter cancellation; group timeout still applies |
| `WithMaxConcurrent` | `DefaultMaxConcurrent` (1024) | Distinct active keys; non-positive disables the limit            |
| `WithPanicAsError`  | `nil`                         | Replace panic values with the supplied error                     |
| `WithTimeout`       | `DefaultTimeout` (15s)        | Shared callback deadline; non-positive retains the default       |

## Key types

| Symbol          | Description                                                                |
|-----------------|----------------------------------------------------------------------------|
| `ErrAborted`    | The callback exited with `runtime.Goexit` instead of returning             |
| `ErrClosed`     | Admission stopped by Close or Wait                                         |
| `ErrOverloaded` | A new key was refused because the group is at `WithMaxConcurrent` capacity |
| `Group[K, V]`   | Shares one computation among callers with equal keys; create it with `New` |

## Usage

```go
group := singleflight.New[string, User]()
user, err := group.Do(ctx, id, func(work context.Context) (User, error) {
    return loadUser(work, id)
})
```

New keys at capacity return `ErrOverloaded`; existing work can still be joined. There is no background admission queue. Callback contexts retain the
first caller's values and use the group deadline. Callbacks must honor cancellation; their capacity remains occupied until they actually exit.

Use [concurrency](../concurrency) for bounded processing of a slice rather than deduplicating calls by key.

## Shared results and lifecycle

`DoShared` returns `(value, shared, error)`. Shared means another caller joined
that execution, even if it later canceled; copy mutable results before modifying
them. Canceled and rejected callers return false. `Do` preserves its original API.

`Forget(key)` detaches a call without canceling it or its waiters. Its execution
still occupies capacity until it exits; later calls may therefore be overloaded.
`Close()` rejects all new calls (including joins) with `ErrClosed`; `Wait()` first
closes admission, then drains all executions, including forgotten ones. Wait
implies Close and is safe while calls are arriving and alongside other Wait calls.
Neither Close nor Wait cancels work. A pre-canceled caller receives its context error first.

By default work keeps the first caller's values and the group timeout; caller
cancellation never cancels it. `WithLifecycle(parent)` additionally cancels work
when parent ends or the last waiter cancels. Last-waiter cancellation detaches the
call, allowing later callers to start fresh subject to capacity. Work retains the
first caller's values, not parent's values. Cancel parent before Close/Wait when
you want shutdown to interrupt work. Callbacks must honor cancellation.

Default panics are retained and re-panicked in waiting callers. Explicit
`WithPanicAsError(err)` returns that error instead, without storing or exposing
the panic value. A nil option retains the default policy. `runtime.Goexit` still
returns `ErrAborted`.

### Migrating from the internal flight helper

The group timeout still applies with `WithLifecycle`: every execution has the
15-second default deadline unless a positive `WithTimeout(duration)` overrides
it. The timeout cannot be disabled; zero and negative values retain the default.
When migrating from `security/secrets/internal/flight`, set `WithTimeout`
explicitly to the application's intended execution bound, alongside
`WithLifecycle(parent)`. Parent or last-waiter cancellation can end work sooner.
The internal helper had no independent group timeout, so merely supplying its
parent context does not preserve its former unbounded execution duration.

For effectively unbounded runs like the internal helper's, combine
`WithLifecycle(parent)` with `WithTimeout(time.Duration(math.MaxInt64))`:

```go
group := singleflight.New[string, User](
    singleflight.WithLifecycle(parent),
    singleflight.WithTimeout(time.Duration(math.MaxInt64)),
)
```

This retains parent and last-waiter cancellation while moving the group deadline
roughly 292 years into the future. It is effectively unbounded for service
lifetimes, but is not literally deadline-free: callbacks still observe a deadline.
