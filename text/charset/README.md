# charset

```go
import "github.com/altessa-s/go-atlas-core/text/charset"
```

Package `charset` decodes bytes to UTF-8: a charset declared in the Content-Type wins, otherwise valid UTF-8 is kept, otherwise a caller-supplied
fallback encoding (for example `charmap.Windows1251`) decodes the bytes.

## Key API

| Symbol              | Description                                                                                     |
|---------------------|-------------------------------------------------------------------------------------------------|
| `Decode`            | `Decode(raw, contentType, fallback)` returns raw as UTF-8 following the decision order below    |
| `ErrUnknownCharset` | A declared charset label is not in the WHATWG encoding index                                    |
| `ErrInvalidUTF8`    | The input is undeclared (or declared UTF-8), not valid UTF-8, and the fallback is nil           |

## Decision order

| Input                                              | Result                                                    |
|----------------------------------------------------|-----------------------------------------------------------|
| Content-Type declares a non-UTF-8 charset          | Decoded with that charset (labels and aliases per WHATWG) |
| Content-Type declares an unknown charset           | Error wrapping `ErrUnknownCharset`                        |
| No charset (or a UTF-8 label), valid UTF-8         | Returned as is, without a leading UTF-8 byte order mark   |
| No charset (or a UTF-8 label), invalid UTF-8       | Decoded with `fallback`; `ErrInvalidUTF8` when it is nil  |
| Content-Type that does not parse as a media type   | Treated as undeclared                                     |

A declared UTF-8 label is deliberately treated as undeclared: legacy servers often label single-byte files `charset=utf-8`, and decoding them
strictly would replace every non-ASCII byte with U+FFFD. The result may share memory with `raw` when no transcoding is needed.

## Usage

```go
body, err := charset.Decode(raw, resp.Header.Get("Content-Type"), charmap.Windows1251)
if err != nil {
    return fmt.Errorf("decode snapshot: %w", err)
}
```
