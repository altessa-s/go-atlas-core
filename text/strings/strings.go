// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package strings

import (
	"crypto/subtle"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/altessa-s/go-atlas-core/types/constraints"
)

// caseInsensitiveRegexCache caches compiled case-insensitive regexes keyed by separator.
// Bounded to maxRegexCacheSize entries to prevent unbounded memory growth.
var caseInsensitiveRegexCache sync.Map // map[string]*regexp.Regexp

// regexCacheSize tracks the approximate number of entries in caseInsensitiveRegexCache.
var regexCacheSize atomic.Int64

// maxRegexCacheSize is the upper bound on cached regex entries. Once reached,
// new separators are compiled but not cached.
const maxRegexCacheSize = 256

// IsEmpty reports whether str is empty or contains only whitespace characters.
// The generic type parameter accepts both string and *string values. A nil
// *string is treated as empty and returns true. Whitespace detection uses
// [strings.TrimSpace], so all Unicode whitespace code points are recognized.
//
// For a non-generic alternative that works only on plain strings, see
// [IsEmptyOrWhitespace].
//
// Example:
//
//	IsEmpty("")        // true
//	IsEmpty("  ")      // true
//	IsEmpty("hello")   // false
func IsEmpty[T ~string | ~*string](str T) bool {
	switch v := any(str).(type) {
	case string:
		return len(strings.TrimSpace(v)) == 0
	case *string:
		if v == nil {
			return true
		}
		return len(strings.TrimSpace(*v)) == 0
	default:
		// This should never happen with the type constraint
		// but we return true for safety (empty by default)
		return true
	}
}

// Pre-instantiated converters for common numeric types. Each variable is a
// specialization of the generic [To] function, avoiding the need for explicit
// type parameters at call sites.
//
// Example:
//
//	n := ToInt64("42")      // int64(42)
//	f := ToFloat64("3.14")  // float64(3.14)
var (
	ToUint    = To[uint]
	ToUint64  = To[uint64]
	ToUint32  = To[uint32]
	ToUint16  = To[uint16]
	ToUint8   = To[uint8]
	ToInt     = To[int]
	ToInt64   = To[int64]
	ToInt32   = To[int32]
	ToInt16   = To[int16]
	ToInt8    = To[int8]
	ToFloat32 = To[float32]
	ToFloat64 = To[float64]
)

// ToPtr converts a string or *string into a *string, returning nil when the
// value is empty or contains only whitespace. For *string inputs, the original
// pointer is returned if it passes the non-empty check; for string inputs a
// new pointer is allocated. Nil *string inputs return nil.
//
// Example:
//
//	ToPtr("hello")  // &"hello"
//	ToPtr("")       // nil
//	ToPtr("  ")     // nil
func ToPtr[T interface{ ~string | ~*string }](str T) *string {
	switch t := any(str).(type) {
	case *string:
		if t == nil || strings.TrimSpace(*t) == "" {
			return nil
		}
		return t
	case string:
		if t == "" || strings.TrimSpace(t) == "" {
			return nil
		}
		return &t
	}
	return nil
}

// FromPtr dereferences ptr and returns the pointed-to string. If ptr is nil,
// the empty string is returned. This is the inverse of [ToPtr].
//
// Example:
//
//	s := "hello"
//	FromPtr(&s)   // "hello"
//	FromPtr(nil)  // ""
func FromPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// To parses str into a numeric value of type T. Supported types are all signed
// and unsigned integer types, as well as float32 and float64 (constrained by
// [constraints.Numbers]). On a parse error the zero value of T is returned
// with no error indication; callers that need error details should use the
// [strconv] package directly.
//
// # Safety of unsafe.Sizeof
//
// The use of `unsafe.Sizeof(t)` is safe because:
//
//  1. Compile-Time Constant: For a given type T, Sizeof returns a compile-time constant.
//     It doesn't access any memory; it merely returns the size of the type in bytes.
//
//  2. No Pointer Dereference: Sizeof operates on the type, not on any value's memory.
//     Even though `t` is the zero value, Sizeof doesn't read from `t`'s address.
//
//  3. Standard Pattern: This is the idiomatic way to get a type's bit size for
//     strconv.Parse* functions in generic code.
//
// Example:
//
//	To[int]("42")       // 42
//	To[float64]("3.14") // 3.14
//	To[int]("invalid")  // 0
func To[T constraints.Numbers](str string) T {
	var t T
	typ := reflect.TypeFor[T]()
	kind := typ.Kind()
	const bitsPerByte = 8
	// Safe: Sizeof returns the size of type T, doesn't access memory
	bitSize := int(unsafe.Sizeof(t) * bitsPerByte)

	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		d, err := strconv.ParseInt(str, 10, bitSize)
		if err != nil {
			return t
		}
		return T(d)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		d, err := strconv.ParseUint(str, 10, bitSize)
		if err != nil {
			return t
		}
		return T(d)
	case reflect.Float32, reflect.Float64:
		d, err := strconv.ParseFloat(str, bitSize)
		if err != nil {
			return t
		}
		return T(d)
	default:
		// The type constraint ensures numeric kinds; keep a safe fallback.
		return t
	}
}

// IsEmptyOrWhitespace reports whether s is zero-length or consists entirely of
// Unicode whitespace code points. Unlike [IsEmpty], it operates only on plain
// strings (no pointer support) and iterates runes directly, returning at the
// first non-whitespace rune instead of trimming both ends.
//
// Example:
//
//	IsEmptyOrWhitespace("")      // true
//	IsEmptyOrWhitespace("  \t")  // true
//	IsEmptyOrWhitespace("a")     // false
func IsEmptyOrWhitespace(s string) bool {
	if len(s) == 0 {
		return true
	}

	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// ToBytesUnsafe converts a string to []byte without copying the underlying data.
//
// WARNING: The returned slice must NEVER be modified. Modifying it causes undefined
// behavior because Go strings are immutable and may be stored in read-only memory.
//
// # Safety Invariants
//
// This function is safe when the caller adheres to the read-only contract:
//
//  1. Immutability Contract: The caller must treat the returned []byte as read-only.
//     Any modification (including via sub-slices) results in undefined behavior.
//
//  2. Go 1.20+ Safety: Uses unsafe.StringData and unsafe.Slice, which are the
//     officially supported functions for this conversion pattern since Go 1.20.
//     These functions are designed specifically for zero-copy string/byte conversions.
//
//  3. Lifetime: The returned slice is valid only as long as the source string is live.
//     If the string is garbage collected, the slice becomes a dangling pointer.
//     In practice, this is safe because the caller typically holds the string.
//
//  4. Empty String Safety: Returns nil for empty strings to avoid creating a slice
//     header pointing to potentially invalid memory.
//
// Use Cases:
//   - Passing string data to APIs that accept []byte for reading (e.g., hash functions)
//   - Avoiding allocation in hot paths where the data is only read
//
// Example:
//
//	b := ToBytesUnsafe("hello")  // []byte{'h','e','l','l','o'}
//
// #nosec G103 -- intentional zero-copy conversion with documented safety contract
func ToBytesUnsafe(str string) []byte {
	if len(str) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(str), len(str))
}

// FromBytesUnsafe converts []byte to string without copying the underlying data.
//
// WARNING: The source byte slice must NOT be modified after this conversion.
// Modifying it would change the "immutable" string, violating Go's string semantics
// and potentially causing data corruption in maps, comparisons, etc.
//
// # Safety Invariants
//
// This function is safe when the caller adheres to the immutability contract:
//
//  1. Post-Conversion Immutability: The source []byte must not be modified after
//     calling this function. The string and slice share the same backing array.
//
//  2. Go 1.20+ Safety: Uses unsafe.SliceData and unsafe.String, which are the
//     officially supported functions for this conversion pattern since Go 1.20.
//
//  3. Common Safe Patterns:
//     - Slice is locally created and immediately converted (never modified again)
//     - Slice comes from a read-only source (e.g., mmap'd file)
//     - Slice ownership is transferred to the string (caller discards slice reference)
//
//  4. Empty Slice Safety: Returns empty string "" for nil or zero-length slices
//     to avoid creating a string header pointing to potentially invalid memory.
//
// Use Cases:
//   - Converting buffers that won't be reused (e.g., one-shot parsing)
//   - Avoiding allocation when returning string from []byte computation
//
// Example:
//
//	s := FromBytesUnsafe([]byte("hello"))  // "hello"
//
// #nosec G103 -- intentional zero-copy conversion with documented safety contract
func FromBytesUnsafe(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ContainsOptions configures the behavior of the [Contains] function,
// controlling case sensitivity, whole-word matching, and occurrence counting.
type ContainsOptions struct {
	CaseSensitive   bool // CaseSensitive when false, performs case-insensitive search.
	MatchWholeWords bool // MatchWholeWords when true, only matches complete words.
	Count           bool // Count when true, returns the number of occurrences instead of just presence.
}

// ContainsResult holds the outcome of a [Contains] search, including whether
// the substring was found, how many times it occurred, and the byte positions
// of each match (populated only when [ContainsOptions.Count] is true).
// When [ContainsOptions.CaseSensitive] is false, Positions are byte offsets
// into the case-folded copy of the haystack; they coincide with offsets into
// the original string whenever lowercasing is length-preserving (always true
// for ASCII).
type ContainsResult struct {
	Found     bool  // Found indicates whether the substring was found.
	Count     int   // Count is the number of occurrences (only set when ContainsOptions.Count is true).
	Positions []int // Positions contains the start positions of all matches (only when Count is true).
}

// Contains searches for substr within s according to opts and returns a
// [ContainsResult]. An empty substr always matches at position 0. When
// [ContainsOptions.CaseSensitive] is false the search is lowercased. When
// [ContainsOptions.MatchWholeWords] is true, matches that are adjacent to
// letters or digits are rejected. When [ContainsOptions.Count] is true, all
// non-overlapping occurrences are found and their positions recorded;
// otherwise the function returns as soon as the first match is confirmed.
//
// Example:
//
//	r := Contains("Hello World", "hello", ContainsOptions{CaseSensitive: false})
//	// r.Found = true
func Contains(s, substr string, opts ContainsOptions) ContainsResult {
	if substr == "" {
		if opts.Count {
			return ContainsResult{Found: true, Count: 1, Positions: []int{0}}
		}
		return ContainsResult{Found: true}
	}

	searchStr := s
	searchSubstr := substr

	if !opts.CaseSensitive {
		searchStr = strings.ToLower(s)
		searchSubstr = strings.ToLower(substr)
	}

	if !opts.Count && !opts.MatchWholeWords {
		// Simple case: just check if it contains the substring
		found := strings.Contains(searchStr, searchSubstr)
		return ContainsResult{Found: found}
	}

	var positions []int
	var count int
	start := 0

	for {
		idx := strings.Index(searchStr[start:], searchSubstr)
		if idx == -1 {
			break
		}

		actualPos := start + idx

		if opts.MatchWholeWords {
			// The boundary check and the advance step must use the same
			// string the index scan ran on: actualPos is a byte offset
			// into searchStr, and when CaseSensitive is false ToLower can
			// change byte lengths (e.g. U+1E9E "ẞ", 3 bytes, folds to
			// U+00DF "ß", 2 bytes), so indexing the original s with it
			// would misalign. Lowercasing never turns a letter or digit
			// into a non-letter (or vice versa), so classifying the
			// neighbors in the folded string is equivalent.
			if !isWholeWordMatch(searchStr, actualPos, len(searchSubstr)) {
				start = actualPos + 1
				continue
			}
		}

		count++
		if opts.Count {
			positions = append(positions, actualPos)
		}

		if !opts.Count {
			// If we don't need count, we can return early
			return ContainsResult{Found: true}
		}

		start = actualPos + len(searchSubstr)
	}

	return ContainsResult{
		Found:     count > 0,
		Count:     count,
		Positions: positions,
	}
}

// isWholeWordMatch checks if a match at given position is a complete word.
// pos and length are byte offsets, so the adjacent character is decoded
// with utf8 rather than indexed directly — a plain rune(s[pos-1]) cast
// would treat UTF-8 continuation bytes as their Latin-1 codepoint values
// (U+0080..U+00BF), which are never letters or digits, and would wrongly
// report the inside of a multi-byte rune as a word boundary.
func isWholeWordMatch(s string, pos, length int) bool {
	// Check character before the match
	if pos > 0 {
		prevChar, _ := utf8.DecodeLastRuneInString(s[:pos])
		if unicode.IsLetter(prevChar) || unicode.IsDigit(prevChar) {
			return false
		}
	}

	// Check character after the match
	endPos := pos + length
	if endPos < len(s) {
		nextChar, _ := utf8.DecodeRuneInString(s[endPos:])
		if unicode.IsLetter(nextChar) || unicode.IsDigit(nextChar) {
			return false
		}
	}

	return true
}

// SecureCompare performs a constant-time comparison of provided and expected
// using [crypto/subtle.ConstantTimeCompare], returning true only when they are
// byte-identical. Use this instead of == when comparing secrets such as
// passwords, API keys, or HMAC digests to prevent timing side-channel attacks.
//
// Note: both strings are converted to []byte, which allocates. For
// repeated comparisons of sensitive data, consider [SecureString.Equal].
//
// Example:
//
//	SecureCompare(userToken, expectedToken)  // true if equal
func SecureCompare(provided, expected string) bool {
	return subtle.ConstantTimeCompare(ToBytesUnsafe(provided), ToBytesUnsafe(expected)) == 1
}

// TimingSafePrefixMatch reports whether s starts with prefix using a
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
//	TimingSafePrefixMatch("Bearer token", "bearer")  // true
func TimingSafePrefixMatch(s, prefix string) bool {
	if len(s) < len(prefix) {
		// Extend s to prefix length to avoid timing leaks
		s += strings.Repeat("\x00", len(prefix)-len(s))
	}

	// Convert both to lowercase for case-insensitive comparison
	sLower := strings.ToLower(s[:len(prefix)])
	prefixLower := strings.ToLower(prefix)

	// Use constant-time comparison
	return subtle.ConstantTimeCompare(ToBytesUnsafe(sLower), ToBytesUnsafe(prefixLower)) == 1
}

// SubstringMatch reports whether s contains substr using a case-insensitive
// comparison.
//
// WARNING: This function is NOT constant-time. The underlying [strings.Contains]
// call reveals match presence and position through timing. For security-sensitive
// searches, use [TimingSafeSubstringMatch] instead.
//
// Example:
//
//	SubstringMatch("Hello World", "world")  // true
func SubstringMatch(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TimingSafeSubstringMatch reports whether s contains substr using a
// constant-time, case-insensitive search. Every possible starting position
// in s is examined with [crypto/subtle.ConstantTimeCompare] so that neither
// the presence nor the position of the match is revealed through timing
// differences. An empty substr always returns true.
//
// The function lowercases both inputs, so it has O(len(s) * len(substr))
// comparison cost. Use this only when timing leaks are a genuine concern.
//
// Example:
//
//	TimingSafeSubstringMatch("Hello World", "world")  // true
func TimingSafeSubstringMatch(s, substr string) bool {
	if substr == "" {
		return true
	}

	sLower := strings.ToLower(s)
	subLower := ToBytesUnsafe(strings.ToLower(substr))
	n := len(subLower)

	if len(sLower) < n {
		return false
	}

	sLowerBytes := ToBytesUnsafe(sLower)
	found := 0
	for i := 0; i <= len(sLowerBytes)-n; i++ {
		found |= subtle.ConstantTimeCompare(sLowerBytes[i:i+n], subLower)
	}
	return found == 1
}

// StringEqualsUnsafe compares two strings for exact (case-sensitive)
// equality with a fast path that checks whether both share the same backing
// memory (common when strings have been interned). Strings of different
// lengths are never equal. For same-length strings with different data
// pointers, a standard == comparison is used.
//
// # Safety Invariants
//
// This function uses unsafe.StringData for pointer comparison optimization:
//
//  1. Read-Only Access: We only compare pointer values, never dereference them.
//     This is equivalent to comparing memory addresses.
//
//  2. Go 1.20+ Safety: unsafe.StringData is the officially supported way to get
//     a string's underlying data pointer since Go 1.20.
//
//  3. String Interning Optimization: Go may intern identical string literals,
//     causing them to share the same backing memory. This check detects that case
//     in O(1) time, avoiding character-by-character comparison.
//
//  4. Correctness: If pointers differ, we fall back to standard string comparison
//     which is always correct. The unsafe path is purely an optimization.
//
// Example:
//
//	StringEqualsUnsafe("hello", "hello")  // true
//
// #nosec G103 -- intentional pointer comparison for performance optimization
func StringEqualsUnsafe(a, b string) bool {
	if len(a) != len(b) {
		// Different lengths can never be byte-equal. The earlier
		// strings.EqualFold fallback made the function case-insensitive
		// for different-length inputs only — a semantic landmine.
		return false
	}

	if len(a) == 0 {
		return true
	}

	// Get string data pointers using modern unsafe functions.
	// This is safe because we only compare addresses, never dereference.
	aPtr := unsafe.StringData(a)
	bPtr := unsafe.StringData(b)

	// If they point to the same memory, they're equal (string interning optimization)
	if aPtr == bPtr {
		return true
	}

	// Fall back to regular comparison
	return a == b
}

// Concat concatenates parts into a single string using a pooled
// [strings.Builder] from [StringBuilderPool]. The builder is pre-grown to the
// exact total length, so only one allocation occurs for the final string.
// Returns the empty string for zero parts and the original string for one part.
//
// Example:
//
//	Concat("hello", " ", "world")  // "hello world"
func Concat(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	builder := GetStringBuilder()
	defer PutStringBuilder(builder)

	totalLen := 0
	for _, part := range parts {
		totalLen += len(part)
	}
	builder.Grow(totalLen)

	for _, part := range parts {
		builder.WriteString(part)
	}

	return builder.String()
}

// ConcatUnsafe concatenates parts into a single string with exactly one heap
// allocation for the combined byte buffer. The returned string is created via
// [FromBytesUnsafe], meaning it shares its backing array with the internal
// buffer. This is safe because the buffer is not retained or modified after
// the function returns.
//
// Returns the empty string for zero parts and the original string for one part.
//
// Example:
//
//	ConcatUnsafe("hello", " ", "world")  // "hello world"
func ConcatUnsafe(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// Calculate total length
	totalLen := 0
	for _, s := range parts {
		totalLen += len(s)
	}

	if totalLen == 0 {
		return ""
	}

	// Allocate once
	result := make([]byte, 0, totalLen)
	for _, s := range parts {
		result = append(result, s...)
	}

	return FromBytesUnsafe(result)
}
