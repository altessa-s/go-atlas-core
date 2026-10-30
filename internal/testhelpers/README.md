# testhelpers

```go
import "github.com/altessa-s/go-atlas-core/internal/testhelpers"
```

Shared test doubles and fixtures for this module. The package depends only on the standard library.

## Key API

| Symbol                       | Description                                                                         |
|------------------------------|-------------------------------------------------------------------------------------|
| `MockReadCloser`             | Wraps a reader and records whether it was closed                                    |
| `MockNetError`               | Configurable network error double                                                   |
| `Person`, `User`             | Sample structs for collection and serialization tests                               |
| `StressDuration`             | Reads a positive duration from `ATLAS_STRESS_DURATION`                              |
| `DefaultStressDuration`      | Default stress-test duration of 30 seconds                                          |
| `VerifyNoLeaks`              | Fails the test if a goroutine it started is leaked when it finishes                 |
| `TrackGoroutines`            | Returns a `GoroutineTracker` for the goroutines the caller starts from now on       |
| `GoroutineTracker.Leaked`    | Stacks of the tracked goroutines the `goroutineleak` profile reports as leaked      |

## Goroutine leak checks

`VerifyNoLeaks` and `GoroutineTracker` read the runtime's `goroutineleak` profile (`runtime/pprof`), which runs a garbage collection that finds
goroutines blocked on a channel or sync primitive no running goroutine can reach. Such a goroutine can never resume. Goroutines that are merely
slow to exit, or blocked on something still reachable (a channel the test holds, a package variable), are not reported, so the check has no
false positives from timing.

Attribution uses pprof labels, which goroutines inherit from their creator: the helper labels the calling goroutine with a unique value and only
reports leaked goroutines carrying it. The check is therefore safe under `t.Parallel`, and a goroutine leaked by one test never fails another.
It replaces any pprof labels already set on the calling goroutine, and does not see goroutines started by a goroutine outside the test (for
example a package-level background loop). Call `VerifyNoLeaks` first so its cleanup runs after the test's own cleanups.

## Usage

```go
reader := &testhelpers.MockReadCloser{Reader: strings.NewReader("data")}
defer reader.Close()
```

```go
func TestWorkerPool(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	// ... start and stop goroutines ...
}
```

Linux subprocess tests use [exectest](../../runtime/internal/exectest) for process isolation.
