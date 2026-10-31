# leakcheck

`Check() error` collects Go 1.27's `goroutineleak` profile, triggering its detecting
GC. It returns nil if the profile is absent or empty, otherwise an error with
stacks. It detects permanently blocked goroutines, not live goroutines still
reachable by work that could release them. A goroutine not yet parked may be missed.

Keep the testing dependency in your test package:

```go
func TestMain(m *testing.M) {
    code := m.Run()
    if err := leakcheck.Check(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        if code == 0 { code = 1 }
    }
    os.Exit(code)
}
```

Checks are process-wide: isolate tests that deliberately leak in a subprocess.
