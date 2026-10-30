# optional

```go
import "github.com/altessa-s/go-atlas-core/types/optional"
```

Package `optional` provides a generic `Optional[T]` value that holds either a value of type `T` or nothing. It makes the "value may be absent" intent
explicit at the type level, for places where Go's idiomatic `(T, bool)` pair or `*T` is awkward — struct fields, channel elements, slice values, map
values.

Internally `Optional[T]` carries `(value, present)` by value: `Some(v)` does **not** allocate, the type is comparable when `T` is comparable, and the
zero value of `Optional[T]` is a valid `None`.

## Functions

| Function      | Description                                                              |
|---------------|--------------------------------------------------------------------------|
| `Some[T]`     | `T` -> `Optional[T]` (carries `v`)                                       |
| `None[T]`     | `()` -> `Optional[T]` (empty)                                            |
| `Of[T]`       | `(T, bool)` -> `Optional[T]` (canonical bridge from `value, ok` lookups) |
| `FromPtr[T]`  | `*T` -> `Optional[T]` (None if nil, Some with dereferenced value)        |
| `ToPtr[T]`    | `Optional[T]` -> `*T` (nil if None, pointer to value if Some)           |

## Methods

| Method                                       | Description                                                          |
|----------------------------------------------|----------------------------------------------------------------------|
| `Get() (T, bool)`                            | Underlying pair; the bridge back to idiomatic Go control flow        |
| `Value() T`                                  | Contained value (zero value of `T` if `Optional` is `None`)          |
| `IsSome() bool`                              | Reports whether the `Optional` carries a value                       |
| `IsNone() bool`                              | Reports whether the `Optional` is empty                              |
| `OrDefault(def T) T`                         | Contained value if `Some`, otherwise `def`                           |
| `OrElse(fn) T`                               | Contained value if `Some`, otherwise `fn()` (only invoked on `None`) |
| `IsZero() bool`                              | `true` when `None`; lets `bson:",omitempty"` strip `None` fields     |
| `MarshalBSONValue() (byte, []byte, error)`   | Rejects unconfigured BSON with `ErrBSONCodecRequired`                |
| `UnmarshalBSONValue(byte, []byte) error`     | Rejects unconfigured BSON without changing the value                 |
| `MarshalJSONTo(*jsontext.Encoder) error`     | Some → `v` in the caller's codec; None → `null` (encoding/json/v2)   |
| `UnmarshalJSONFrom(*jsontext.Decoder) error` | JSON `null` → None; otherwise decode into `T` and store as Some      |

## Reflection helpers

For codecs and serializers that detect `Optional` fields by reflection; ordinary code should rely on the type system.

| Function                    | Description                                                          |
|-----------------------------|----------------------------------------------------------------------|
| `IsOptionalType(t)`         | Reports whether `t` is an `Optional[T]` declared by this package     |
| `InnerType(optType)`        | Type parameter `T`; panics on a non-`Optional` type                  |
| `GetReflect(opt)`           | `(value, present)` pair from a `reflect.Value` of `Optional[T]`      |
| `SomeReflect(optType, v)`   | Build `Some(v)` as a `reflect.Value` of `optType`                    |
| `NoneReflect(optType)`      | Build `None` as a `reflect.Value` of `optType`                       |

## Serialization

JSON support is built in: `Some(v)` encodes as `v`, `None` as `null`. Use `json:",omitzero"` to omit None fields with Go 1.25+. Present zero values
remain present.

JSON support is implemented with the `encoding/json/v2` interfaces `MarshalJSONTo` / `UnmarshalJSONFrom`; the type has no `MarshalJSON` /
`UnmarshalJSON` methods. Both `encoding/json` and `encoding/json/v2` call them, and they encode `T` with the options of the calling codec: under
`encoding/json` the bytes are exactly those the former `MarshalJSON` produced; under `encoding/json/v2`, `Some(v)` encodes and decodes like a bare `v`
does in v2 — a nil slice as `[]`, invalid UTF-8 rejected, pointer-receiver marshalers of `T` called. Code that type-asserts `json.Marshaler` /
`json.Unmarshaler` no longer matches `Optional`; call `json.Marshal` / `json.Unmarshal` (either version) instead.

`Optional` does not implement BSON hooks itself. A BSON codec for it must be registered by the caller's Mongo registry and set on the Mongo client
options or standalone BSON encoders and decoders. Without it, present Optional fields return `ErrBSONCodecRequired`; this prevents silent
serialization as empty documents. None with `bson:",omitempty"` is omitted, and `Some(nil)` decodes as None because both encode as BSON null. This
module does not import the Mongo driver.

## When to use

- Struct fields where the zero value of `T` is itself a valid value and you need to distinguish it from "not set" — e.g. `Optional[string]` to tell
  `Some("")` from `None`.
- Channel/slice/map element types where `nil` would be ambiguous or where `T` is not nillable.
- Function parameters whose presence carries semantic weight you want the type signature to advertise.

## When NOT to use

- **Ordinary lookups.** Keep the `value, ok := m[k]` and `value, ok := <-ch` idioms — wrapping each in `Optional` adds noise.
- **Pointer fields where `nil` already means "absent".** `*T` is fine when callers cannot construct `Some(zero)` legitimately.
- **"Value or error" cases.** Use [`types/result`](../result/README.md), not `Optional[T]` plus a side-channel error.
- **Functional pipelines (`Map`, `AndThen`, `Then`, …).** In Go they read as nested closures rather than terse pipelines, so plain `if ok` stays shorter
  and clearer. Convert back with `Get` and use ordinary control flow.

## Usage

```go
type User struct {
    Name     string
    Nickname optional.Optional[string]
}

u := User{Name: "Ada", Nickname: optional.Some("")}
if nick, ok := u.Nickname.Get(); ok {
    fmt.Println("nickname is set:", nick) // distinguishes Some("") from None
}

// Bridge from a (value, ok) lookup.
v, ok := m[k]
opt := optional.Of(v, ok)
host := opt.OrDefault("localhost")

// Convert from/to pointers.
var ptr *string
opt = optional.FromPtr(ptr) // None
ptr = optional.ToPtr(optional.Some("hello")) // &"hello"
```
