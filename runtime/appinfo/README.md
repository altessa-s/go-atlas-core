# appinfo

```go
import "github.com/altessa-s/go-atlas-core/runtime/appinfo"
```

Package `appinfo` provides standardized access to application metadata, build information, environment variables, and directory paths. Values are set
via `-ldflags` at build time and accessed through package-level variables and helper functions.

## Build configuration

```bash
go build -ldflags "
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.Name=MyApp'
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.Version=1.2.3'
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.Project=MyProject'
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.Commit=abc123'
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'
  -X 'github.com/altessa-s/go-atlas-core/runtime/appinfo.EnvPrefix=MYAPP'
"
```

## Globals

| Variable                                 | Description                                       | Default                    |
|------------------------------------------|---------------------------------------------------|----------------------------|
| `Name`                                   | Application name                                  | Module path tail           |
| `Project`                                | Umbrella project name                             | `""`                       |
| `Version`                                | Semantic version string                           | `"0.0.0"`                  |
| `Commit`                                 | Abbreviated VCS revision (6 chars, uppercase)     | From `vcs.revision`        |
| `Branch`                                 | VCS branch name                                   | `""`                       |
| `BuildTime`                              | Build timestamp                                   | From `vcs.time`            |
| `EnvPrefix`                              | Env var prefix (uppercased, `_`-separated)        | `""`                       |
| `StandaloneLabel` / `NonStandaloneLabel` | Labels returned by `EnvLabel`                     | `"standalone"` / `"cloud"` |
| `IsStandalone`                           | `true` when built with the `standalone` build tag | `false`                    |

If `Name` is not set via ldflags, `init` derives it from the last path component of the module path, falling back to `"unknown"` if build info is
unavailable. `IsStandalone` is a compile-time constant selected by the presence of the `standalone` build tag (`go build -tags standalone`), not a
runtime-assignable variable.

## Version checks

| Function             | Description                                    |
|----------------------|------------------------------------------------|
| `IsRelease`          | Stable release (no pre-release identifiers)    |
| `IsPreRelease`       | Has pre-release identifiers                    |
| `IsAlpha`            | Version is `0.0.0` or has `alpha` pre-release  |
| `IsBeta`             | Has `beta` pre-release                         |
| `IsReleaseCandidate` | Has `rc` pre-release                           |
| `SemVersion`         | Parsed `SemanticVersion` struct (shallow copy) |

```go
type SemanticVersion struct {
    Major      string   // e.g. "1"
    Minor      string   // e.g. "2"
    Patch      string   // e.g. "3"
    Prerelease []string // e.g. ["alpha", "1"] for "1.2.3-alpha.1"
}
```

`SemVersion()` returns a shallow copy: the scalar fields are safe to modify, but `Prerelease` shares its backing array with the package-level
value — clone it (`slices.Clone`) before mutating.

## Build info

| Function                    | Description                                                                 |
|-----------------------------|-----------------------------------------------------------------------------|
| `AppVersion`                | Multi-line version summary for `--version`                                  |
| `Info`                      | Compact one-liner: version, revision, prefix                                |
| `BuildInfo`                 | Compact one-liner: Go, platform, build time, tags                           |
| `BuildGoVersion`            | Go toolchain version                                                        |
| `BuildPlatform`             | `GOOS/GOARCH`                                                               |
| `BuildGoOS` / `BuildGoArch` | `GOOS` / `GOARCH` build settings                                            |
| `BuildTags`                 | Active build tags                                                           |
| `IsCgoEnabled`              | `CGO_ENABLED` build setting                                                 |
| `IsRaceEnabled`             | Built with `-race`                                                          |
| `EnvLabel`                  | `StandaloneLabel` or `NonStandaloneLabel`                                   |
| `Deps`                      | Module dependencies (`Dependencies` with `Contains` / `Get` by module path) |

`Info()` returns `"version=X.Y.Z, revision=ABCDEF, env_prefix=PREFIX"`; `BuildInfo()` returns `"go=X.Y.Z, platform=os/arch, date=<BuildTime>,
tags=tag1,tag2"` (`date` is the raw `BuildTime` value, not reformatted).

```go
deps := appinfo.Deps()
if deps.Contains("github.com/example/mod") {
    dep := deps.Get("github.com/example/mod")
    fmt.Printf("version=%s replaced=%v\n", dep.Version, dep.IsReplaced)
}
```

## Environment variables

| Function                | Description                                                 |
|-------------------------|-------------------------------------------------------------|
| `EnvKeyPrefix`          | `EnvPrefix` as a name prefix: `MYAPP_` (or `""`)            |
| `NormalizeEnvKeyPrefix` | Same normalization for any prefix (used by `config/loader`) |
| `GetEnvVar`             | Read `<EnvPrefix>_<KEY>` with `~` expansion                 |
| `Env`                   | Read env var (thin `os.Getenv` wrapper)                     |
| `EnvOr`                 | Read env var with a default                                 |
| `EnvCached`             | Read env var with thread-safe caching                       |
| `ClearEnvCache`         | Invalidate all cached entries                               |
| `HomeDir`               | User home directory (cross-platform)                        |
| `ExpandPath`            | Replace leading `~` with home directory                     |

`EnvPrefix` is bare (`MYAPP`). Pass `EnvKeyPrefix()` to the configuration loader so it reads the same `MYAPP_*` names as `GetEnvVar`:

```go
loader.New(nil, loader.WithEnvPrefix(appinfo.EnvKeyPrefix()))
```

`EnvCached` uses double-checked locking (`sync.RWMutex`) so repeated hot-path lookups of a variable that does not change at runtime avoid
re-reading the environment on every call; call `ClearEnvCache` in tests or after modifying environment variables.

`HomeDir` checks `USERPROFILE` first, then `HOMEDRIVE` + `HOMEPATH`, then falls back to `HOME`; it returns `""` if none are set.

### Well-known constants

The package defines constants for commonly used environment variable names, consumed by `Env`, `EnvOr`, `EnvCached`, and `HomeDir`:

| Constant             | Value                  |
|----------------------|------------------------|
| `EnvHome`            | `"HOME"`               |
| `EnvUserProfile`     | `"USERPROFILE"`        |
| `EnvHomeDrive`       | `"HOMEDRIVE"`          |
| `EnvHomePath`        | `"HOMEPATH"`           |
| `EnvNATSURL`         | `"NATS_URL"`           |
| `EnvAnthropicAPIKey` | `"ANTHROPIC_API_KEY"`  |
| `EnvVaultToken`      | `"VAULT_TOKEN"`        |

## Directory paths

| Function        | Default path                    |
|-----------------|---------------------------------|
| `BinDir`        | `/opt/bin`                      |
| `VarDir`        | `/var`                          |
| `EtcDir`        | `/etc/<project>/<name>`         |
| `LibDir`        | `<VarDir>/lib/<project>/<name>` |
| `CertsCacheDir` | `<LibDir>/certs`                |
| `MakeAllDirs`   | Create `VarDir` and `LibDir`    |

All paths are overridable via `<EnvPrefix>_*_DIR` environment variables (`BIN_DIR`, `VAR_DIR`, `ETC_DIR`, `LIB_DIR`). `<project>` and `<name>` are
lowercased, and the project component is omitted when `Project` is empty. Paths are resolved on every call, so environment changes take effect
immediately.

`MakeAllDirs` creates `VarDir` and `LibDir` (and any necessary parents) with `os.MkdirAll` at permission `0777`; the process umask applies, so the
actual on-disk mode is narrower in practice.

With `Project = "myplatform"` and `Name = "myservice"`:

```
/etc/myplatform/myservice/     ← EtcDir()
/var/lib/myplatform/myservice/ ← LibDir()
/var/                          ← VarDir()
/opt/bin/                      ← BinDir()
```
