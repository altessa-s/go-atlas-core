// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// JoinOptions configures the behavior of the [Join] function, controlling the
// separator placed between elements, optional prefix/suffix wrapping, and
// whether empty strings are excluded from the output.
type JoinOptions struct {
	Separator string // Separator is the string to place between elements.
	SkipEmpty bool   // SkipEmpty when true, excludes empty strings from joining.
	Prefix    string // Prefix is prepended to the result if it's not empty.
	Suffix    string // Suffix is appended to the result if it's not empty.
}

// Join concatenates elements into a single string using the rules specified in
// opts. When [JoinOptions.SkipEmpty] is true, empty strings are removed before
// joining. If the joined result is non-empty and [JoinOptions.Prefix] or
// [JoinOptions.Suffix] are set, they are prepended and appended respectively.
// Returns the empty string when elements is empty or all elements are empty
// with SkipEmpty enabled.
//
// Example:
//
//	parts := []string{"a", "", "b"}
//	Join(parts, JoinOptions{Separator: ","})                  // "a,,b"
//	Join(parts, JoinOptions{Separator: ",", SkipEmpty: true}) // "a,b"
func Join(elements []string, opts JoinOptions) string {
	if len(elements) == 0 {
		return ""
	}

	var filteredElements []string
	if opts.SkipEmpty {
		for _, elem := range elements {
			if elem != "" {
				filteredElements = append(filteredElements, elem)
			}
		}
	} else {
		filteredElements = elements
	}

	if len(filteredElements) == 0 {
		return ""
	}

	result := strings.Join(filteredElements, opts.Separator)
	if result != "" {
		result = opts.Prefix + result + opts.Suffix
	}

	return result
}

// SplitOptions configures the behavior of [Split] and [SplitSeq], controlling
// the separator, maximum number of splits, whitespace trimming, empty-element
// removal, and case sensitivity.
type SplitOptions struct {
	Separator     string // Separator is the string to split on.
	MaxSplits     int    // MaxSplits limits the number of splits. -1 means no limit.
	TrimSpace     bool   // TrimSpace when true, trims whitespace from each resulting part.
	SkipEmpty     bool   // SkipEmpty when true, excludes empty strings from the result.
	CaseSensitive bool   // CaseSensitive when false, performs case-insensitive splitting.
}

// Split divides s into substrings around occurrences of
// [SplitOptions.Separator] and returns the resulting slice. The behavior is
// further controlled by [SplitOptions]: case-insensitive matching, a maximum
// number of splits, per-element whitespace trimming, and removal of empty
// elements.
//
// When [SplitOptions.CaseSensitive] is false, a separator occurrence is any
// run of runes that matches the separator under Unicode simple case folding
// (the rule used by [strings.EqualFold]), so "ſ" matches "s" and the Kelvin
// sign matches "k". Pure ASCII input and separator use a fast lowercase scan;
// other input uses a compiled case-insensitive regular expression that is
// cached for subsequent calls. A separator that is not valid UTF-8 is always
// matched exactly.
//
// Returns nil when s is empty and [SplitOptions.SkipEmpty] is true. For a
// lazy, allocation-free alternative see [SplitSeq].
//
// Example:
//
//	Split("a,b,c", SplitOptions{Separator: ","})                      // ["a", "b", "c"]
//	Split("a,,b", SplitOptions{Separator: ",", SkipEmpty: true})      // ["a", "b"]
func Split(s string, opts SplitOptions) []string {
	if s == "" {
		if opts.SkipEmpty {
			return nil
		}
		return []string{""}
	}

	var parts []string
	separator := opts.Separator

	if !opts.CaseSensitive && separator != "" {
		sp := caseInsensitiveSplitter(s, separator, opts.MaxSplits)
		for part, ok := sp.next(); ok; part, ok = sp.next() {
			parts = append(parts, part)
		}
	} else {
		// Case-sensitive splitting (standard behavior)
		if opts.MaxSplits <= 0 {
			parts = strings.Split(s, separator)
		} else {
			parts = strings.SplitN(s, separator, opts.MaxSplits+1)
		}
	}

	// Process the parts based on options
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if opts.TrimSpace {
			part = strings.TrimSpace(part)
		}
		if opts.SkipEmpty && part == "" {
			continue
		}
		result = append(result, part)
	}

	if len(result) == 0 && opts.SkipEmpty {
		return nil
	}

	return result
}

// splitter splits s around separator occurrences, performing at most
// maxSplits splits when maxSplits is positive. It is shared by [Split] and
// [SplitSeq] and is driven by direct calls to [splitter.next], so the hot loop
// does not go through a function value.
//
// Occurrences are located either with strings.Index of sep in hay (exact
// matching uses hay == s; the ASCII case-insensitive path uses the lowercased
// input and separator, whose offsets equal those in s) or, when re is
// non-nil, with re on s.
type splitter struct {
	s         string         // s is the input being split.
	hay       string         // hay is searched for sep when re is nil.
	sep       string         // sep is the separator searched in hay.
	re        *regexp.Regexp // re locates separators in s when non-nil.
	maxSplits int            // maxSplits limits the number of splits when positive.
	start     int            // start is the offset of the next part.
	splits    int            // splits is the number of separators consumed so far.
	done      bool           // done reports whether the final part has been returned.
}

// exactSplitter returns a [splitter] for byte-exact matches of sep.
func exactSplitter(s, sep string, maxSplits int) splitter {
	return splitter{s: s, hay: s, sep: sep, maxSplits: maxSplits}
}

// caseInsensitiveSplitter returns the [splitter] shared by [Split] and
// [SplitSeq] for case-insensitive matching under Unicode simple case folding.
// sep must be non-empty.
func caseInsensitiveSplitter(s, sep string, maxSplits int) splitter {
	if lowSep, ok := lowerASCII(sep); ok {
		if lowS, ok := lowerASCII(s); ok {
			// ASCII lowercasing is byte-for-byte, so offsets in the lowered
			// input are offsets in s, and for ASCII text lowercase comparison
			// equals simple case folding.
			return splitter{s: s, hay: lowS, sep: lowSep, maxSplits: maxSplits}
		}
	} else if !utf8.ValidString(sep) {
		return exactSplitter(s, sep, maxSplits)
	}

	re, err := caseInsensitiveRegex(sep)
	if err != nil {
		return exactSplitter(s, sep, maxSplits)
	}
	return splitter{s: s, re: re, maxSplits: maxSplits}
}

// next returns the next part and true, or "" and false when all parts have
// been returned.
func (sp *splitter) next() (string, bool) {
	if sp.done {
		return "", false
	}
	if sp.maxSplits <= 0 || sp.splits < sp.maxSplits {
		lo, hi := -1, -1
		if sp.re == nil {
			if idx := strings.Index(sp.hay[sp.start:], sp.sep); idx >= 0 {
				lo = sp.start + idx
				hi = lo + len(sp.sep)
			}
		} else if loc := sp.re.FindStringIndex(sp.s[sp.start:]); loc != nil {
			lo = sp.start + loc[0]
			hi = sp.start + loc[1]
		}
		if lo >= 0 {
			part := sp.s[sp.start:lo]
			sp.start = hi
			sp.splits++
			return part, true
		}
	}
	sp.done = true
	return sp.s[sp.start:], true
}

// caseInsensitiveRegex returns a compiled "(?i)" regular expression matching
// sep literally, using the bounded caseInsensitiveRegexCache.
func caseInsensitiveRegex(sep string) (*regexp.Regexp, error) {
	if cached, ok := caseInsensitiveRegexCache.Load(sep); ok {
		return cached.(*regexp.Regexp), nil //nolint:errcheck // sync.Map guarantees correct type
	}

	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(sep))
	if err != nil {
		return nil, err
	}
	// Optimistic insert, then enforce bound. Load-then-store would
	// race: multiple goroutines could all observe size < max and
	// each add 1, overshooting the bound by up to GOMAXPROCS.
	if _, loaded := caseInsensitiveRegexCache.LoadOrStore(sep, re); !loaded {
		if regexCacheSize.Add(1) > maxRegexCacheSize {
			caseInsensitiveRegexCache.Delete(sep)
			regexCacheSize.Add(-1)
		}
	}
	return re, nil
}

// splitRunes splits s after each UTF-8 sequence exactly like
// strings.SplitN(s, "", maxSplits+1): invalid bytes form their own element
// and, when maxSplits is positive, the unsplit remainder is the last element.
// Splitting stops early when emit returns false. s must be non-empty.
func splitRunes(s string, maxSplits int, emit func(string) bool) {
	for n := 0; s != ""; n++ {
		if maxSplits > 0 && n == maxSplits {
			emit(s)
			return
		}
		_, size := utf8.DecodeRuneInString(s)
		if !emit(s[:size]) {
			return
		}
		s = s[size:]
	}
}

// lowerASCII returns s with ASCII letters lowercased and true when s is pure
// ASCII, or "" and false otherwise. The ASCII check and the uppercase scan
// share one pass, and, like [strings.ToLower], s itself is returned without
// allocating when it has no uppercase letters.
func lowerASCII(s string) (string, bool) {
	hasUpper := false
	for i := range len(s) {
		c := s[i]
		if c >= utf8.RuneSelf {
			return "", false
		}
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}
	if !hasUpper {
		return s, true
	}

	b := make([]byte, len(s))
	for i := range len(s) {
		b[i] = asciiToLower(s[i])
	}
	// b is not referenced after the conversion, so sharing its memory is safe.
	return FromBytesUnsafe(b), true
}

// isASCII reports whether s contains only ASCII bytes.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
