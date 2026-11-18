# semver

```go
import "github.com/altessa-s/go-atlas-core/semver"
```

Parse SemVer 2.0.0 versions with an optional lowercase `v` prefix. All three numeric components are required. Whitespace is invalid.

## Usage

| API                     | Behavior                                                               |
|-------------------------|------------------------------------------------------------------------|
| `Parse(s)`              | Returns a `Version` with major, minor, patch, prerelease and build data |
| `version.Compare(other)` | Returns −1, 0 or 1 by precedence, ignoring build metadata              |

For the running application's build version, see [appinfo](../runtime/appinfo/).
