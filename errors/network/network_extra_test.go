// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package network_test

import (
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/altessa-s/go-atlas-core/errors/network"
)

// TestIsConnectionRefused_SyscallErrorWrap pins the realistic error shape
// produced by net.Dial on a refused connection: the errno is wrapped in
// *os.SyscallError inside *net.OpError, not stored as a bare syscall.Errno.
func TestIsConnectionRefused_SyscallErrorWrap(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: os.NewSyscallError("connect", syscall.ECONNREFUSED),
	}
	if !network.IsConnectionRefused(opErr) {
		t.Error("should detect ECONNREFUSED wrapped in *os.SyscallError inside *net.OpError")
	}
	if !network.IsConnectionRefused(&url.Error{Op: "Get", URL: "http://x", Err: opErr}) {
		t.Error("should detect ECONNREFUSED through a *url.Error layer")
	}
}

func TestIsURLError(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "http://x", Err: errors.New("fail")}
	if !network.IsURLError(urlErr) {
		t.Error("should detect url.Error")
	}
	if network.IsURLError(errors.New("plain")) {
		t.Error("should not detect plain error")
	}
	if network.IsURLError(nil) {
		t.Error("should not detect nil")
	}
}

func TestIsCertUnknownAuthority(t *testing.T) {
	// Non-url.Error should not match
	if network.IsCertUnknownAuthority(errors.New("other")) {
		t.Error("should not detect plain error")
	}
	if network.IsCertUnknownAuthority(nil) {
		t.Error("should not detect nil")
	}
	// url.Error with non-cert inner error
	urlErr := &url.Error{Op: "Get", URL: "https://x", Err: errors.New("other")}
	if network.IsCertUnknownAuthority(urlErr) {
		t.Error("should not detect url.Error with plain inner error")
	}
}

func TestIsNetworkError_Nil(t *testing.T) {
	if network.IsNetworkError(nil) {
		t.Error("should not detect nil")
	}
}

func TestIsRequestTimeoutError_NonTimeout(t *testing.T) {
	if network.IsRequestTimeoutError(nil) {
		t.Error("should not detect nil")
	}
	if network.IsRequestTimeoutError(errors.New("plain")) {
		t.Error("should not detect plain error")
	}
}

func TestIsResourceRedirects_Nil(t *testing.T) {
	if network.IsResourceRedirects(nil) {
		t.Error("should not detect nil")
	}
	if network.IsResourceRedirects(errors.New("no redirects")) {
		t.Error("should not detect non-redirect error")
	}
}

func TestIsUnsupportedProtocolScheme_Nil(t *testing.T) {
	if network.IsUnsupportedProtocolScheme(nil) {
		t.Error("should not detect nil")
	}
}

func TestIsConnectionRefused_Nil(t *testing.T) {
	if network.IsConnectionRefused(nil) {
		t.Error("should not detect nil")
	}
	if network.IsConnectionRefused(errors.New("other")) {
		t.Error("should not detect plain error")
	}
}
