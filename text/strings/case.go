// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package strings

import (
	"unicode"
	"unicode/utf8"
)

// IsLowercaseUnsafe reports whether every letter in s is lowercase, using a
// fast byte-level ASCII scan via [ToBytesUnsafe]. If any non-ASCII byte is
// encountered, it falls back to the allocation-free [IsLowercase] for correct
// Unicode handling. Empty strings return true.
//
// Example:
//
//	IsLowercaseUnsafe("hello")  // true
//	IsLowercaseUnsafe("Hello")  // false
func IsLowercaseUnsafe(s string) bool {
	return isCaseUnsafe(s, 'A', 'Z', IsLowercase)
}

// IsLowercase reports whether every letter in s (as determined by
// [unicode.IsLetter]) is lowercase. Non-letter runes are ignored. Empty
// strings return true.
//
// Example:
//
//	IsLowercase("hello123")  // true
//	IsLowercase("Hello")     // false
func IsLowercase(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// IsUppercaseUnsafe reports whether every letter in s is uppercase, using a
// fast byte-level ASCII scan via [ToBytesUnsafe]. If any non-ASCII byte is
// encountered, it falls back to the allocation-free [IsUppercase] for correct
// Unicode handling. Empty strings return true.
//
// Example:
//
//	IsUppercaseUnsafe("HELLO")  // true
//	IsUppercaseUnsafe("Hello")  // false
func IsUppercaseUnsafe(s string) bool {
	return isCaseUnsafe(s, 'a', 'z', IsUppercase)
}

func isCaseUnsafe(s string, asciiBadMin, asciiBadMax byte, fallback func(string) bool) bool {
	if len(s) == 0 {
		return true
	}

	// Fast path for ASCII-only strings
	const asciiThreshold = 0x80 // Characters >= 0x80 are non-ASCII
	bytes := ToBytesUnsafe(s)
	for _, b := range bytes {
		if b >= asciiThreshold {
			return fallback(s)
		}
		if b >= asciiBadMin && b <= asciiBadMax {
			return false
		}
	}
	return true
}

// IsUppercase reports whether every letter in s (as determined by
// [unicode.IsLetter]) is uppercase. Non-letter runes are ignored. Empty
// strings return true.
//
// Example:
//
//	IsUppercase("HELLO123")  // true
//	IsUppercase("Hello")     // false
func IsUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

// IsASCIISpace reports whether b is one of the six ASCII whitespace bytes:
// space (0x20), horizontal tab (0x09), newline (0x0A), vertical tab (0x0B),
// form feed (0x0C), or carriage return (0x0D). It does not recognize Unicode
// whitespace code points beyond ASCII.
//
// Example:
//
//	IsASCIISpace(' ')   // true
//	IsASCIISpace('\t')  // true
//	IsASCIISpace('a')   // false
func IsASCIISpace(b byte) bool {
	// Create a bitmask for ASCII space characters
	// ' '(32), '\t'(9), '\n'(10), '\r'(13), '\v'(11), '\f'(12)
	return b == 32 || (b >= 9 && b <= 13 && b != 11) || b == 11
}

// IsTrimmed reports whether s has no leading or trailing whitespace. For
// ASCII-only strings the check uses [IsASCIISpace]; for strings with non-ASCII
// first or last characters the rune is decoded with [utf8.DecodeRuneInString]
// and tested with [unicode.IsSpace]. Returns true for the empty string.
//
// Example:
//
//	IsTrimmed("hello")   // true
//	IsTrimmed(" hello")  // false
//	IsTrimmed("")        // true
func IsTrimmed(s string) bool {
	const MaxASCIICode = 128

	if len(s) == 0 {
		return true
	}

	// Check first and last characters for whitespace
	firstRune := rune(s[0])
	lastRune := rune(s[len(s)-1])

	// For ASCII characters, we can check directly
	if firstRune < MaxASCIICode && lastRune < MaxASCIICode {
		return !IsASCIISpace(byte(firstRune)) && !IsASCIISpace(byte(lastRune))
	}

	// For non-ASCII, use utf8 package to decode runes without allocation
	firstRune, _ = utf8.DecodeRuneInString(s)
	lastRune, _ = utf8.DecodeLastRuneInString(s)
	return !unicode.IsSpace(firstRune) && !unicode.IsSpace(lastRune)
}

// IsTrimmedUnsafe reports whether s has no leading or trailing whitespace,
// using [ToBytesUnsafe] for zero-copy byte-level access. For ASCII bytes the
// check uses [IsASCIISpace]; for non-ASCII bytes the first/last rune is
// decoded with [utf8.DecodeRune] and tested with [unicode.IsSpace]. Returns
// true for the empty string.
//
// WARNING: Assumes the input is valid UTF-8 for non-ASCII characters. Invalid
// sequences may produce incorrect results.
//
// Example:
//
//	IsTrimmedUnsafe("hello")  // true
//	IsTrimmedUnsafe(" hi ")   // false
func IsTrimmedUnsafe(s string) bool {
	if len(s) == 0 {
		return true
	}

	// Get direct access to string bytes
	bytes := ToBytesUnsafe(s)

	// Check the first byte for ASCII whitespace
	const asciiThreshold = 0x80 // Characters >= 0x80 are non-ASCII
	firstByte := bytes[0]
	if firstByte < asciiThreshold { // ASCII
		if IsASCIISpace(firstByte) {
			return false
		}
	} else {
		// Non-ASCII: decode first rune manually using unsafe access
		firstRune, _ := utf8.DecodeRune(bytes)
		if unicode.IsSpace(firstRune) {
			return false
		}
	}

	// Check last byte for ASCII whitespace
	lastByte := bytes[len(bytes)-1]
	if lastByte < asciiThreshold { // ASCII
		return !IsASCIISpace(lastByte)
	}

	// Non-ASCII: decode last rune manually using unsafe access
	lastRune, _ := utf8.DecodeLastRune(bytes)
	return !unicode.IsSpace(lastRune)
}

// TrimSuffixFast removes suffix from the end of s if present, using direct
// substring comparison rather than [strings.TrimSuffix]. Returns s unchanged
// when suffix is empty, s is shorter than suffix, or suffix is not found.
// Because Go strings are backed by shared memory, the returned substring never
// allocates.
//
// Example:
//
//	TrimSuffixFast("hello/", "/")    // "hello"
//	TrimSuffixFast("hello", "/")     // "hello" (no allocation)
//	TrimSuffixFast("test.go", ".go") // "test"
func TrimSuffixFast(s, suffix string) string {
	if len(suffix) == 0 || len(s) < len(suffix) {
		return s
	}
	if s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// TrimPrefixFast removes prefix from the beginning of s if present, using
// direct substring comparison rather than [strings.TrimPrefix]. Returns s
// unchanged when prefix is empty, s is shorter than prefix, or prefix is not
// found. Because Go strings are backed by shared memory, the returned
// substring never allocates.
//
// Example:
//
//	TrimPrefixFast("/hello", "/")     // "hello"
//	TrimPrefixFast("hello", "/")      // "hello" (no allocation)
//	TrimPrefixFast("test.go", "test") // ".go"
func TrimPrefixFast(s, prefix string) string {
	if len(prefix) == 0 || len(s) < len(prefix) {
		return s
	}
	if s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
