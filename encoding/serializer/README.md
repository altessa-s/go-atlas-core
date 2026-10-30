# serializer

```go
import "github.com/altessa-s/go-atlas-core/encoding/serializer"
```

Package `serializer` defines a format-agnostic `Serializer` interface for encoding and decoding Go values. It is used by `data/cache`,
`data/idempotency`, `data/saga`, and other packages that need pluggable marshaling without a hard dependency on a specific encoding format.

The interface has two methods: `Serialize(data any) ([]byte, error)` and `Deserialize(d []byte, out any) error`, where `out` must be a non-nil pointer.
Implementations must be stateless and safe for concurrent use. The zero value of `JSON` is ready to use without any initialization and applies the `encoding/json/v2` defaults.

## Implementations

| Type                    | Backend            | Notes                                                     |
|-------------------------|--------------------|-----------------------------------------------------------|
| `JSON` / `NewJSON(...)` | `encoding/json/v2` | Zero value applies the v2 defaults; options via `NewJSON` |

## JSON

`JSON` delegates to `encoding/json/v2`. `NewJSON(opts ...json.Options)` takes the standard library's own `json.Options` / `jsontext.Options` values
and passes them to every `Marshal` / `Unmarshal` call (later options win; marshal-only options are ignored when decoding and vice versa).

**Behavior change.** `JSON` used to delegate to `encoding/json` (v1). The v2 defaults are stricter, which matters for existing stores and caches:

| Rule                     | Before (`encoding/json`) | Now (v2 default)           | Relax with                                                         |
|--------------------------|--------------------------|----------------------------|--------------------------------------------------------------------|
| Invalid UTF-8 in strings | Replaced with U+FFFD     | Error on encode and decode | `jsontext.AllowInvalidUTF8(true)`                                  |
| Duplicate member names   | Last one wins            | `*jsontext.SyntacticError` | `jsontext.AllowDuplicateNames(true)`                               |
| Struct field names       | Case-insensitive match   | Case-sensitive match       | `json.MatchCaseInsensitiveNames(true)`                             |
| Nil slice / nil map      | `null`                   | `[]` / `{}`                | `json.FormatNilSliceAsNull(true)`, `json.FormatNilMapAsNull(true)` |
| HTML characters          | Escaped (`\u003c`)       | Written as-is              | `jsontext.EscapeForHTML(true)`                                     |

Data written before the change may fail to decode (invalid UTF-8, duplicate names) or decode differently (member names that only matched
case-insensitively); data written now decodes with `encoding/json`. Passing `DefaultOptionsV1()` from `encoding/json` restores the v1 behavior in full:

```go
import (
    jsonv1 "encoding/json"
    "encoding/json/v2"
)

s := serializer.NewJSON(json.Deterministic(true))
legacy := serializer.NewJSON(jsonv1.DefaultOptionsV1()) // byte-identical to the former JSON

data, err := s.Serialize(&user)
var loaded User
err = s.Deserialize(data, &loaded)
```
