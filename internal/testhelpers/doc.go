// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package testhelpers provides the test doubles and fixtures shared by the
// go-atlas-core packages: a network error double ([MockNetError]), an I/O
// double ([MockReadCloser]), sample structs ([Person], [User]), the stress
// test duration knob ([StressDuration]) and goroutine leak checks
// ([VerifyNoLeaks], [GoroutineTracker]) built on the runtime's goroutineleak
// profile.
//
// The package depends only on the standard library.
package testhelpers
