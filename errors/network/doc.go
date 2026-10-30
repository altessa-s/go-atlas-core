// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package network classifies network, URL and TLS errors produced by the
// standard library's net, net/http and crypto/x509 packages.
//
// It lives apart from the parent errors package so that callers needing only
// error wrapping and canonical sentinels do not pull in net, net/url,
// crypto/x509, regexp and syscall.
//
// All functions are stateless, nil-safe and thread-safe. They traverse the
// full error chain, so errors wrapped with %w (including the parent package's
// Wrap helpers) are still classified.
//
// # Classifiers
//
// [IsNetworkError], [IsRequestTimeoutError] and [IsConnectionRefused] inspect
// transport failures; [IsURLError], [IsResourceRedirects],
// [IsUnsupportedProtocolScheme] and [IsCertUnknownAuthority] inspect the
// [*net/url.Error] values returned by an HTTP client.
//
//	resp, err := client.Do(req)
//	switch {
//	case network.IsConnectionRefused(err):
//		return errors.ErrUnavailable
//	case network.IsRequestTimeoutError(err):
//		return retry()
//	}
package network
