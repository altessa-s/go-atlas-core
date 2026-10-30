# text

Text processing utilities for the Atlas framework. Each subpackage can be imported independently. `strings` has no external dependencies;
`charset` depends only on `golang.org/x/text`.

## Subpackages

| Package                          | Description                                                                |
|----------------------------------|----------------------------------------------------------------------------|
| [charset](charset)               | Decode bytes to UTF-8: declared charset, then valid UTF-8, then a fallback |
| [strings](strings)               | String manipulation, case transformation, truncation, join/split, pooling  |
| [strings/intern](strings/intern) | String interning with LFU eviction and an injectable clock                 |
| [strings/secure](strings/secure) | Zeroing secret storage, in-place zeroing, timing-safe comparisons          |
