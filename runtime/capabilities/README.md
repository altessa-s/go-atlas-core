# capabilities

```go
import "github.com/altessa-s/go-atlas-core/runtime/capabilities"
```

A minimal, cgo-free Go wrapper around the Linux capabilities subsystem (`capset(2)`, `capget(2)`, `prctl(2)` with `PR_CAPBSET_*` / `PR_CAP_AMBIENT_*`).
It lets a process drop capabilities at startup and confine itself to a strict allowlist without pulling in libcap.

Any process can reduce its effective, permitted, inheritable and ambient sets without privilege. Dropping bits from the bounding set needs
`CAP_SETPCAP` (root has it, including in a default Docker container); `DropAll`, `DropAllExcept` and `Apply` keep it effective until the bounding
drops are done and reject a bounding drop without it with `ErrInvalidOption` before changing anything. A permitted bit can be kept but never added, and
the bounding set limits only what can be added — the standard Linux rules, not something this package works around.

## API

| Symbol | Purpose |
|---|---|
| `Cap` | Strongly-typed capability bit (one constant per kernel `CAP_*`) |
| `Sets` | Snapshot of all five capability sets (effective/permitted/inheritable/bounding/ambient) |
| `SetKind` | Names one set: `Effective`, `Permitted`, `Inheritable`, `Bounding`, `Ambient` |
| `(Sets).Has(c Cap, kinds ...SetKind)` | Report whether `c` is in any of the named sets (Effective when none given) |
| `(Sets).With(c Cap, kinds ...SetKind)` | Copy of the snapshot with `c` added to the named sets |
| `Get()` | Read the current snapshot from the calling thread |
| `DropAll()` | Drop every capability everywhere — the "nuclear option" |
| `DropAllExcept(keep ...Cap)` | Drop every capability except those listed; a listed capability the thread does not hold is skipped, not an error |
| `Apply(s Sets)` | Install an exact `Sets` snapshot on the calling thread — a multi-step syscall sequence (not atomic); impossible requests are rejected before any change, and steps that succeeded stay in effect when a later one fails, so inspect the partial state on error |
| `BoundingDrop(c Cap)` | Remove `c` from the bounding set (irreversible) |
| `AmbientClearAll()` | Clear the ambient set in one call |
| `AmbientRaise(c Cap)` | Add `c` to the ambient set |
| `AmbientLower(c Cap)` | Remove `c` from the ambient set |
| `ParseName(string) (Cap, error)` | Resolve a canonical `"CAP_*"` string (case-insensitive) |
| `(Cap).String()` | Canonical `"CAP_*"` name |
| `ErrUnsupported` | Platform doesn't support Linux capabilities (non-Linux) |
| `ErrFailed` | The capset/capget/prctl syscall failed (sentinel wrapping the kernel errno via `%w`) |
| `ErrInvalidOption` | Operator supplied a bad capability name or an impossible set |

## Quick start

### Nuclear option — drop everything

```go
package main

import (
    "log"

    "github.com/altessa-s/go-atlas-core/runtime/capabilities"
)

func main() {
    if err := capabilities.DropAll(); err != nil {
        log.Fatalf("capabilities: %v", err)
    }
    run()
}
```

### Bind a privileged port then drop everything else

```go
// Bind first, while CAP_NET_BIND_SERVICE is still effective.
ln, err := net.Listen("tcp", ":443")
if err != nil { log.Fatal(err) }

// Drop everything except what we still need.
if err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE); err != nil {
    log.Fatalf("capabilities: %v", err)
}

serve(ln)
```

### Resolve operator config from YAML

```go
var keep []capabilities.Cap
for _, name := range cfg.Sandbox.Capabilities.Keep {
    c, err := capabilities.ParseName(name)
    if err != nil {
        return fmt.Errorf("sandbox.capabilities.keep: %w", err)
    }
    keep = append(keep, c)
}
if err := capabilities.DropAllExcept(keep...); err != nil {
    return err
}
```

## The four-set model (short version)

Linux capabilities are not a single mask but five related sets tracked per thread. Most operator use cases only care about the first three:

| Set | Who uses it | How to change it |
|---|---|---|
| Effective | Kernel permission checks at syscall time | `capset(2)` |
| Permitted | Ceiling from which Effective can be raised | `capset(2)` |
| Bounding | Ceiling from which Permitted can be raised | `prctl(PR_CAPBSET_DROP)` — lowering only |
| Inheritable | Passed across non-privileged execve | `capset(2)` |
| Ambient | Preserved across non-privileged execve | `prctl(PR_CAP_AMBIENT_*)` |

If you do not know which set you need, you want effective/permitted/bounding. `DropAllExcept` handles exactly those three; inheritable and ambient are
cleared (safe default for a long-running service). A kept capability stays only where the thread already holds it: in bounding if it is in the bounding
set, effective and permitted if it is permitted.

`Apply` runs lower-ambient → capset → bounding drops → capset → raise-ambient. The first capset keeps `CAP_SETPCAP` effective for the bounding drops and
the second removes it when the snapshot does not keep it. Before any of it, `Apply` checks the snapshot against the rules the kernel applies (see
[Caveats](#caveats)) and rejects one that breaks them with `ErrInvalidOption`, leaving the thread untouched. A capset the kernel still refuses runs
before the irreversible bounding drops and leaves only ambient bits lowered; a failed bounding drop leaves the requested effective/permitted/inheritable
installed and the bounding set partially reduced, and the error says how far it got.

## When to use it

- **Long-running services** launched with effective capabilities they no longer need after startup (typical: a service started by systemd with
  `AmbientCapabilities=CAP_NET_BIND_SERVICE`, which binds :443 and then drops the cap).
- **Plugin hosts** where plugins loaded via `dlopen` would otherwise inherit the host's caps.
- **CLI tools** running with file capabilities that want to drop them before invoking untrusted subcommands.
- **As one layer in the hardening sequence:** `nonewprivs` → `rlimits` → `capabilities` → `seccomp` → `landlock`.

## Caveats

- **Linux only.** On non-Linux platforms every function returns `ErrUnsupported` so cross-platform code can import the package without build tags.
- **Per-thread scope.** `capset(2)` mutates only the calling thread. The package pins the goroutine to its OS thread during `Apply`, `DropAll` and
  `DropAllExcept` (`runtime.LockOSThread`), but the change lands on that thread only — even at startup the Go runtime already runs peer OS threads that
  keep their capabilities, and the plugin sandbox is subject to the same limit. For a process-wide drop, drop capabilities externally before the Go
  binary starts (systemd `CapabilityBoundingSet=`, container `--cap-drop`, or a launcher that drops them before `execve(2)`). The pin ends when the call
  returns, and `Get`, `BoundingDrop` and the ambient helpers do not pin at all, so consecutive calls may land on different threads: wrap a sequence that
  builds on earlier results (`Get` then `Apply`, `BoundingDrop` then `DropAllExcept`, `Apply` then `Get`) in `runtime.LockOSThread`.
- **Irreversible ceilings.** Bits dropped from the bounding set cannot be raised by this thread or any descendant. Once you call `BoundingDrop` or
  `DropAllExcept`, you cannot take it back.
- **Raising is bounded.** `Apply` rejects with `ErrInvalidOption`, before changing anything, a snapshot that breaks a rule the kernel enforces: a
  permitted bit not currently permitted, an effective bit outside the requested permitted mask, an inheritable bit outside the current inheritable and
  bounding sets (or, without `CAP_SETPCAP` effective, outside the current inheritable and permitted sets), an ambient bit outside the requested
  permitted and inheritable masks, or a bounding drop without `CAP_SETPCAP` permitted. The kernel would refuse these with `EPERM`, some of them only
  after the irreversible bounding drops. Effective and permitted bits outside the bounding set are allowed: after `BoundingDrop(CAP_NET_BIND_SERVICE)`
  the capability stays permitted, and `DropAllExcept(CAP_NET_BIND_SERVICE)` keeps it.
- **Defense in depth, not isolation.** A malicious dependency already loaded in-process can still read every byte of memory in the host's address space.
  Capability dropping closes the "ambient privileged syscalls" vector only.

## Smoke test (manual)

Because this package's mutating calls are irreversible, they are not unit-tested in-process; `capabilities_subprocess_linux_test.go`
exercises them in a re-executed child process. For manual syscall coverage, build this 40-line harness and run it on a Linux 5.x+ host:

```go
package main

import (
    "fmt"
    "log"
    "net"

    "github.com/altessa-s/go-atlas-core/runtime/capabilities"
)

func main() {
    before, err := capabilities.Get()
    must(err)
    fmt.Printf("before: %+v\n", before)

    must(capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE))

    after, err := capabilities.Get()
    must(err)
    fmt.Printf("after:  %+v\n", after)

    ln80, err := net.Listen("tcp", ":80")
    if err != nil {
        log.Fatalf("listen :80: %v (expected success)", err)
    }
    _ = ln80.Close()

    must(capabilities.DropAll())

    if _, err := net.Listen("tcp", ":81"); err == nil {
        log.Fatal("listen :81: got nil, want permission denied")
    } else {
        fmt.Printf("listen :81: %v (expected)\n", err)
    }
}

func must(err error) {
    if err != nil {
        log.Fatal(err)
    }
}
```

Run with `go run smoke.go` (the binary must have `cap_net_bind_service=p` set, e.g. via `sudo setcap 'cap_net_bind_service=+ep' ./smoke`).

## See also

- [`capabilities(7)` man page](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- `runtime/nonewprivs` — `PR_SET_NO_NEW_PRIVS` primitive, first step in the hardening sequence
- `runtime/rlimits` — resource caps, second step
- `runtime/landlock` — filesystem allowlist, final step
