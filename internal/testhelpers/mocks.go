// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package testhelpers

// MockNetError implements the [net.Error] interface for testing.
// The exported fields control the return values of the corresponding methods:
// Msg is returned by Error, IsTimeout by Timeout, and IsTemp by Temporary.
type MockNetError struct {
	IsTimeout bool
	IsTemp    bool
	Msg       string
}

// Error returns Msg.
func (e *MockNetError) Error() string { return e.Msg }

// Timeout returns IsTimeout.
func (e *MockNetError) Timeout() bool { return e.IsTimeout }

// Temporary returns IsTemp.
func (e *MockNetError) Temporary() bool { return e.IsTemp }
