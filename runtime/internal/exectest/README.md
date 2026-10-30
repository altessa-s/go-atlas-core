# exectest

```go
import "github.com/altessa-s/go-atlas-core/runtime/internal/exectest"
```

Runs Linux security tests in a child process to isolate irreversible syscall effects. Tests are skipped on other platforms.

## Key API

| Symbol            | Description                                                           |
|-------------------|-----------------------------------------------------------------------|
| `RunInSubprocess` | Re-executes the test binary and checks child execution and exit status |
| `Failf`           | Reports a child failure and exits without running deferred functions   |

## Usage

```go
func TestDropAll(t *testing.T) {
    t.Parallel()
    exectest.RunInSubprocess(t, func() {
        if err := capabilities.DropAll(); err != nil {
            exectest.Failf("DropAll: %v", err)
        }
    })
}
```

Use `Failf` inside the callback to report failures to the parent. Keep Linux syscall tests in `_linux_test.go` files.
General test doubles live in [testhelpers](../../../internal/testhelpers).
