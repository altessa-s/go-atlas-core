# network

```go
import "github.com/altessa-s/go-atlas-core/errors/network"
```

Package `network` classifies network, URL and TLS errors from `net`, `net/http` and `crypto/x509`. It is split from the parent [`errors`](../)
package so that wrapping and sentinels stay free of the `net`, `crypto/x509`, `regexp` and `syscall` dependencies. All functions are nil-safe and walk
the full error chain.

## Classifiers

| Function                       | Detects                                          |
|--------------------------------|--------------------------------------------------|
| `IsNetworkError`               | Any `net.Error`                                  |
| `IsRequestTimeoutError`        | `net.Error` or `*url.Error` with `Timeout()`     |
| `IsURLError`                   | Any `*url.Error`                                 |
| `IsConnectionRefused`          | `syscall.ECONNREFUSED` through URL/net layers    |
| `IsResourceRedirects`          | "stopped after N redirects"                      |
| `IsUnsupportedProtocolScheme`  | "unsupported protocol scheme"                    |
| `IsCertUnknownAuthority`       | `x509.UnknownAuthorityError` inside `*url.Error` |

## Usage

```go
resp, err := client.Do(req)
switch {
case network.IsConnectionRefused(err):
	return errors.ErrUnavailable
case network.IsRequestTimeoutError(err):
	return retry()
}
```
