# runtime

```go
import "github.com/altessa-s/go-atlas-core/runtime"
```

Package `runtime` provides shutdown hook registries — one process-wide, plus scoped groups for components that outlive neither.

## Functions

| Function           | Description                                                     |
|--------------------|-----------------------------------------------------------------|
| `OnShutdown`       | Register a process-wide shutdown hook (LIFO order)              |
| `RunShutdownHooks` | Execute all process-wide hooks exactly once; errors are joined  |

## Key types

| Type           | Description                                                                        |
|----------------|------------------------------------------------------------------------------------|
| `HookGroup`    | An independently runnable set of shutdown hooks; zero value ready, safe concurrent |
| `ShutdownHook` | `func(ctx context.Context) error` registered via `OnShutdown`                      |

```go
// Register in startup order — db first, cache second.
// They execute in reverse: cache flushes, then db closes.
runtime.OnShutdown(func(ctx context.Context) error {
    return db.Close()
})

runtime.OnShutdown(func(ctx context.Context) error {
    return cache.Flush(ctx)
})

// Trigger during shutdown (e.g. from a signal handler's coordinator):
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := runtime.RunShutdownHooks(ctx); err != nil {
    logger.Error("shutdown errors", slog.Any("error", err))
}
```

A typical application wires this into a signal handler's shutdown coordinator so `RunShutdownHooks` runs on `SIGTERM`/`SIGINT` — see
[signals](signals) for the handler side of that integration, and [panics](panics) for the matching behavior on an unrecovered goroutine panic
(`panics.Handle` runs the shutdown hooks before re-panicking, when re-panic is enabled).

## Shutdown scopes

`OnShutdown` / `RunShutdownHooks` register into one implicit group whose lifetime is the **process**: it runs at most once, and a component that
registered there cannot be stopped on its own. That is right for a resource that lives as long as the program, and wrong for a background component
owned by a factory, a test, or a subsystem that is torn down and rebuilt — those need a scope of their own.

`HookGroup` is that scope. It has the same semantics inside its own boundary — LIFO, at most once, errors joined, a failing hook does not stop the rest
— but each group runs independently:

```go
type Subsystem struct {
    hooks runtime.HookGroup // zero value is ready to use
}

func (s *Subsystem) Start() error {
    s.hooks.OnShutdown(s.engine.Stop)
    s.hooks.OnShutdown(s.storage.Close)
    return nil
}

// Stop tears down only this subsystem; the process keeps running.
func (s *Subsystem) Stop(ctx context.Context) error {
    return s.hooks.Shutdown(ctx) // storage.Close, then engine.Stop
}
```

Hooks registered while `Shutdown` is running are not executed: the set is snapshotted up front, so a self-registering hook cannot extend the sequence it
is part of.

## Subpackages

| Package                      | Description                                                    |
|------------------------------|----------------------------------------------------------------|
| [appinfo](appinfo)           | Application metadata, build info, env vars, directory paths    |
| [capabilities](capabilities) | Linux capability dropping via capset(2) / prctl(2)             |
| [concurrency](concurrency)   | Adaptive concurrency limits, batch processing                  |
| [cycle](cycle)               | Exclusive manual or external cycle driver                      |
| [helpers](helpers)           | Goroutine ID extraction (debug only)                           |
| [landlock](landlock)         | Linux Landlock LSM filesystem allowlist (kernel 5.13+)         |
| [leakcheck](leakcheck)       | Process-wide goroutine leak reporting                          |
| [nonewprivs](nonewprivs)     | `PR_SET_NO_NEW_PRIVS` primitive (SUID-escalation defense)      |
| [panics](panics)             | Panic recovery, assertions (`Must`, `MustNonNil`, `MustError`) |
| [rlimits](rlimits)           | Linux process resource caps via setrlimit(2)                   |
| [seccomp](seccomp)           | Linux seccomp-BPF denylist of 23 dangerous syscalls            |
| [signals](signals)           | OS signal handling with priority, timeouts, graceful shutdown  |
| [singleflight](singleflight) | Cancellable, bounded deduplication of concurrent calls by key  |

## Hardening

The Linux security primitives in this tree — `nonewprivs`, `rlimits`, `capabilities`, `seccomp`, `landlock` — reduce the blast radius of a
compromised plugin or dependency inside an otherwise-trusted process. Each package is independently usable; composed together at process startup,
in the order below, they lock down a Go binary to roughly the surface an operator explicitly granted.

> **Defense in depth, not isolation.** These primitives do not replace a container, VM, or hypervisor: a malicious in-process dependency can still
> read every byte of the host's address space. Pair them with systemd, a container runtime, or a microVM for real isolation.

> **Linux only.** Every package returns `ErrUnsupported` on non-Linux platforms (and `seccomp` also on Linux architectures other than amd64/arm64),
> so the same hardening code compiles and runs unchanged on a developer macOS workstation and a Linux production host — it degrades to a no-op
> where the kernel cannot back it.

### Platform support

| Package        | Minimum kernel | Architectures | Build tags                    |
|----------------|-----------------|----------------|--------------------------------|
| `nonewprivs`   | Linux 3.5       | any            | `linux` / `!linux`             |
| `rlimits`      | Linux (any)     | any            | `linux` / `!linux`             |
| `capabilities` | Linux (any)     | any            | `linux` / `!linux`             |
| `seccomp`      | Linux 3.17      | amd64, arm64   | `linux && (amd64 \|\| arm64)`  |
| `landlock`     | Linux 5.13      | any            | `linux` / `!linux`             |

### Sequence and rationale

Apply the primitives from cheapest/most-recoverable to most irreversible, once at process startup, before spawning goroutines or loading plugins:

```
nonewprivs → rlimits → capabilities → seccomp → landlock
```

1. **`nonewprivs`** runs first. It is the prerequisite for both `seccomp(2)` and `landlock_restrict_self(2)` on unprivileged callers (the alternative is
   `CAP_SYS_ADMIN`, which production hosts should not have) — and both of those packages call `nonewprivs.Set` internally anyway, so running it
   explicitly first just keeps the failure mode legible.
2. **`rlimits`** runs next. `setrlimit(2)` is process-wide and side-effect-light: a misconfigured limit (rejected by validation) does not leave behind
   any partial state in the higher-cost steps below.
3. **`capabilities`** runs after rlimits so the privileged caps a binary actually needs (e.g. `CAP_NET_BIND_SERVICE` to bind port 80) are still in
   effect during that privileged work, and dropped cleanly afterwards — it should only run once the cheap passes are confirmed working.
4. **`seccomp`** runs after capabilities because the seccomp filter denylist may include capability-related syscalls (`unshare`, `setns`, etc.);
   installing the filter after capabilities means a later capability check cannot accidentally re-permit a denied syscall via privilege.
5. **`landlock`** runs last. The filesystem allowlist is the most irreversible step (no relaxation for the lifetime of the calling task) and depends on
   the `NO_NEW_PRIVS` set in step 1.

A sandboxing host composes these packages directly in this order; seccomp is often kept as a separate, explicit step (e.g. calling
`seccomp.BlockDangerousSyscalls()` during host startup) so an audited denylist stays stable across deployments.

### Composing all five

```go
func harden() {
    if err := nonewprivs.Set(); err != nil && !errors.Is(err, nonewprivs.ErrUnsupported) {
        log.Fatalf("nonewprivs: %v", err)
    }

    if err := rlimits.Apply(
        rlimits.WithMemoryBytes(512<<20), // 512 MiB virtual address space
        rlimits.WithMaxOpenFiles(4096),
        rlimits.WithMaxProcesses(256),
        rlimits.WithDisableCoreDumps(), // no post-crash secret leaks
    ); err != nil && !errors.Is(err, rlimits.ErrUnsupported) {
        log.Fatalf("rlimits: %v", err)
    }

    if err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE); err != nil &&
        !errors.Is(err, capabilities.ErrUnsupported) {
        log.Fatalf("capabilities: %v", err)
    }

    if err := seccomp.BlockDangerousSyscalls(); err != nil && !errors.Is(err, seccomp.ErrUnsupported) {
        log.Fatalf("seccomp: %v", err)
    }

    if err := landlock.Apply(
        landlock.WithReadPaths("/etc/myservice", "/lib", "/lib64", "/usr/lib", "/usr/lib64"),
        landlock.WithReadWritePaths("/var/log/myservice"),
    ); err != nil && !errors.Is(err, landlock.ErrUnsupported) {
        log.Fatalf("landlock: %v", err)
    }
}
```

See each package's README for its own options, caveats, and worked examples: [nonewprivs](nonewprivs), [rlimits](rlimits),
[capabilities](capabilities), [seccomp](seccomp), [landlock](landlock).

### Error handling

Every package in the sequence exposes the same error-handling shape, so one `switch` pattern works across all five:

```go
switch {
case err == nil:
    // success
case errors.Is(err, pkg.ErrUnsupported):
    // platform (or, for seccomp, architecture) can't honor this; continue without the protection
case errors.Is(err, pkg.ErrInvalidOption):
    // operator gave us bad config (negative rlimit, bad capability name, non-absolute path); fail fast
case errors.Is(err, pkg.ErrFailed):
    // kernel rejected a well-formed request; the wrap chain preserves the raw syscall.Errno for errors.As
}
```

### What these primitives do not do

- **Not a container replacement.** A namespace-isolated PID/mount/network view is out of scope. Use a container runtime, systemd unit isolation
  (`PrivateTmp=`, `ProtectSystem=`, `PrivateNetwork=`), or a microVM.
- **Not a memory-safety boundary.** A malicious dependency already loaded in-process can read every byte of the host's address space, including
  secrets, TLS keys, and connection state. These packages close specific escalation vectors (container escape, kernel-module loading, ptrace, bpf,
  writing to disk outside an allowlist), not the in-process code-execution vector itself.
- **Not a network sandbox.** None of these packages restrict outbound connections. Use Linux network namespaces, eBPF, an egress firewall, or an L7
  proxy.
- **Not retroactive across goroutines.** `nonewprivs`, `capabilities`, and `landlock` restrict only the calling OS thread; the Go runtime has
  typically already created several peer threads (sysmon, GC, netpoll, GOMAXPROCS workers) by the time `main` runs, and those threads keep their
  original, unrestricted state. `seccomp` is the exception — it uses `SECCOMP_FILTER_FLAG_TSYNC` to propagate the filter to every thread in the
  thread group. For a real process-wide guarantee on the per-thread primitives, apply them externally before the Go binary starts: a systemd unit
  (`NoNewPrivileges=`, `CapabilityBoundingSet=`, `LimitNOFILE=`), a container runtime (`--cap-drop`, `--security-opt=no-new-privileges`), or a small
  C launcher that calls the appropriate `prctl`/`capset`/`landlock_restrict_self` before `execve(2)` of the Go binary.
