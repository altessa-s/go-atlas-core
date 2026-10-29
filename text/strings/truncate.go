// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package strings

import "unicode/utf8"

// TruncateRunes shortens s to at most n runes, marking a cut with suffix.
//
// The suffix counts toward the budget: when s is longer than n runes, the
// result is the first n-k runes of s followed by suffix, where k is the rune
// count of suffix, so the result is exactly n runes long. When n is smaller
// than the suffix, the result is the first n runes of s without the suffix.
// A string of at most n runes is returned unchanged, and an empty suffix cuts
// without a mark. n <= 0 yields "". Whitespace is not trimmed; callers trim
// before truncating when they need to.
//
// Runes are counted as [utf8.RuneCountInString] counts them: each byte of an
// invalid UTF-8 sequence is one rune and is copied as is.
func TruncateRunes(s string, n int, suffix string) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		// A string never has more runes than bytes.
		return s
	}
	keep, mark := n-utf8.RuneCountInString(suffix), suffix
	if keep < 0 {
		keep, mark = n, ""
	}
	// One pass that stops at rune n: cut is the offset of rune keep (keep <= n,
	// so it is set by the time rune n is reached).
	cut, i := 0, 0
	for off := range s {
		if i == keep {
			cut = off
		}
		if i == n {
			return s[:cut] + mark
		}
		i++
	}
	return s
}

// TruncateBytes shortens s to at most n bytes without splitting a UTF-8
// sequence, marking a cut with suffix.
//
// The suffix counts toward the budget: when s is longer than n bytes, the
// result is the longest prefix of s that fits in n-len(suffix) bytes and ends
// on a rune boundary, followed by suffix, so the result never exceeds n
// bytes. When n is smaller than len(suffix), the result is the longest such
// prefix that fits in n bytes, without the suffix. A string of at most n bytes
// is returned unchanged, and an empty suffix cuts without a mark. n <= 0
// yields "". Whitespace is not trimmed; callers trim before truncating when
// they need to.
//
// Only a valid multi-byte sequence is kept whole; a byte of an invalid
// sequence is treated as a one-byte rune.
func TruncateBytes(s string, n int, suffix string) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n < len(suffix) {
		return s[:runeBoundary(s, n)]
	}
	return s[:runeBoundary(s, n-len(suffix))] + suffix
}

// runeBoundary returns the largest offset <= cut (cut < len(s)) that does not
// split a valid UTF-8 sequence of s.
func runeBoundary(s string, cut int) int {
	if utf8.RuneStart(s[cut]) {
		return cut
	}
	// s[cut] is a continuation byte: look back for the leading byte of the
	// sequence it may belong to.
	for j := cut - 1; j >= 0 && j > cut-utf8.UTFMax; j-- {
		if !utf8.RuneStart(s[j]) {
			continue
		}
		if r, size := utf8.DecodeRuneInString(s[j:]); (r != utf8.RuneError || size > 1) && j+size > cut {
			return j
		}
		return cut
	}
	return cut
}
