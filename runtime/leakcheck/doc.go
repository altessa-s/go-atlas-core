// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package leakcheck reports permanently blocked goroutines using the Go runtime's
// goroutineleak profile. Checks are process-wide; run after tests have finished.
package leakcheck
