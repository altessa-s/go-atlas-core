// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package secure

import (
	"crypto/subtle"
	"strings"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

// Equal performs a constant-time comparison of provided and expected
// using [crypto/subtle.ConstantTimeCompare], returning true only when they are
// byte-identical. Use this instead of == when comparing secrets such as
// passwords, API keys, or HMAC digests to prevent timing side-channel attacks.
//
// Note: both strings are converted to []byte, which allocates. For
// repeated comparisons of sensitive data, consider [String.Equal].
//
// Example:
//
//	Equal(userToken, expectedToken)  // true if equal
func Equal(provided, expected string) bool {
	return subtle.ConstantTimeCompare(corestrings.ToBytesUnsafe(provided), corestrings.ToBytesUnsafe(expected)) == 1
}

// PrefixMatch reports whether s starts with prefix using a
// constant-time, case-insensitive comparison via
// [crypto/subtle.ConstantTimeCompare]. If s is shorter than prefix, it is
// padded with null bytes so that the comparison length does not leak through
// timing.
//
// Use this when verifying security-relevant prefixes (e.g. authorization
// scheme headers) where a timing side-channel could reveal the expected value.
//
// Example:
//
//	PrefixMatch("Bearer token", "bearer")  // true
func PrefixMatch(s, prefix string) bool {
	if len(s) < len(prefix) {
		// Extend s to prefix length to avoid timing leaks
		s += strings.Repeat("\x00", len(prefix)-len(s))
	}

	// Convert both to lowercase for case-insensitive comparison
	sLower := strings.ToLower(s[:len(prefix)])
	prefixLower := strings.ToLower(prefix)

	// Use constant-time comparison
	return subtle.ConstantTimeCompare(corestrings.ToBytesUnsafe(sLower), corestrings.ToBytesUnsafe(prefixLower)) == 1
}

// SubstringMatch reports whether s contains substr using a
// constant-time, case-insensitive search. Every possible starting position
// in s is examined with [crypto/subtle.ConstantTimeCompare] so that neither
// the presence nor the position of the match is revealed through timing
// differences. An empty substr always returns true.
//
// The function lowercases both inputs, so it has O(len(s) * len(substr))
// comparison cost. Use this only when timing leaks are a genuine concern;
// otherwise prefer [github.com/altessa-s/go-atlas-core/text/strings.SubstringMatch].
//
// Example:
//
//	SubstringMatch("Hello World", "world")  // true
func SubstringMatch(s, substr string) bool {
	if substr == "" {
		return true
	}

	sLower := strings.ToLower(s)
	subLower := corestrings.ToBytesUnsafe(strings.ToLower(substr))
	n := len(subLower)

	if len(sLower) < n {
		return false
	}

	sLowerBytes := corestrings.ToBytesUnsafe(sLower)
	found := 0
	for i := 0; i <= len(sLowerBytes)-n; i++ {
		found |= subtle.ConstantTimeCompare(sLowerBytes[i:i+n], subLower)
	}
	return found == 1
}
