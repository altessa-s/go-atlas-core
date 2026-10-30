// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package network_test

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/altessa-s/go-atlas-core/errors/network"
)

// refusedDial mirrors the shape net/http returns for a refused connection,
// wrapped once more the way call sites add context on the way up.
var refusedDial = fmt.Errorf("fetch config: %w", &url.Error{
	Op:  "Get",
	URL: "http://example.com",
	Err: &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)},
})

var redirectsErr = fmt.Errorf("fetch config: %w", &url.Error{
	Op:  "Get",
	URL: "http://example.com",
	Err: errors.New("stopped after 10 redirects"),
})

func BenchmarkIsNetworkError(b *testing.B) {
	for b.Loop() {
		_ = network.IsNetworkError(refusedDial)
	}
}

func BenchmarkIsRequestTimeoutError(b *testing.B) {
	for b.Loop() {
		_ = network.IsRequestTimeoutError(refusedDial)
	}
}

func BenchmarkIsConnectionRefused(b *testing.B) {
	for b.Loop() {
		_ = network.IsConnectionRefused(refusedDial)
	}
}

func BenchmarkIsResourceRedirects(b *testing.B) {
	for b.Loop() {
		_ = network.IsResourceRedirects(redirectsErr)
	}
}

func BenchmarkIsUnsupportedProtocolScheme(b *testing.B) {
	for b.Loop() {
		_ = network.IsUnsupportedProtocolScheme(redirectsErr)
	}
}
