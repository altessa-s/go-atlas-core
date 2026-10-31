# cycle

`New(fn)` binds one function to a concurrency-safe guard. `Run(ctx)` invokes it
manually. `Entry()` obtains its guarded entry point without changing ownership.
Pass that function to the external owner, then call `MarkManaged()` only after
registration succeeds. Rejected registration leaves manual runs allowed.
`IsManaged()` reports ownership; marking is permanent and idempotent.
`Managed()` is a convenience for `MarkManaged()` followed by `Entry()` when
registration cannot fail. Later manual calls return `ErrManaged`.

`TryRun(ctx)` and the entry point bypass the ownership check while sharing the
overlap guard. Internal drivers can use them before or after marking.
`TryRunFunc(ctx, fn)` uses that same guard for a per-call closure without replacing
the bound function. For example, an outbox notification driver needs to capture
its fetched count to decide whether to wake itself again; the periodic driver
still uses the bound function. All entry points share one running flag.

Overlapping invocations return nil immediately, without executing or sharing the
running invocation's result. An admitted manual invocation may finish during
handoff. Cancellation is passed to the function, not checked by the guard.
Panics propagate unchanged after releasing the guard. The driver can defer
`panics.Handle(ctx)` to apply the application's recovery policy. The zero value
is not usable; construct with `New` and do not copy after use.
