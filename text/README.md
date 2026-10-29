# text

Text processing utilities for the Atlas framework. Each subpackage can be imported independently. `strings` has no external dependencies;
`charset` depends only on `golang.org/x/text`.

## Subpackages

| Package              | Description                                                                  |
|----------------------|------------------------------------------------------------------------------|
| [charset](charset) | Decode bytes to UTF-8: declared charset, then valid UTF-8, then a fallback   |
| [strings](strings) | String manipulation, case transformation, interning, secure storage, pooling |
