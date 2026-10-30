// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package secure handles sensitive string data: zeroing-on-clear storage,
// best-effort in-place zeroing, and timing-safe comparisons.
//
// # Storage
//
// [String] holds a secret (password, token, key) in storage it owns, zeroes
// it on [String.Clear], stores values up to
// [DefaultSmallStringOptimizationThreshold] bytes inline, and recycles
// instances through an internal pool. It is safe for concurrent reads.
//
// # Zeroing
//
// [ZeroBytes] overwrites a byte slice; [ZeroString] is a best-effort overwrite
// of a string's backing memory that is a silent no-op on read-only data.
//
// # Comparison
//
// [Equal], [PrefixMatch], and [SubstringMatch] compare in constant time so the
// result does not leak through timing.
//
// # Usage
//
//	ss := secure.NewString("password")
//	defer ss.Clear() // explicitly zero memory
//
//	if secure.Equal(provided, expected) {
//		// authorized
//	}
package secure
