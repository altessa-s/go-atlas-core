// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package strings_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

const ellipsis = "…" // one rune, three bytes

func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		n      int
		suffix string
		want   string
	}{
		{name: "empty", s: "", n: 3, suffix: ellipsis, want: ""},
		{name: "zero limit", s: "abc", n: 0, suffix: ellipsis, want: ""},
		{name: "negative limit", s: "abc", n: -1, suffix: ellipsis, want: ""},
		{name: "within limit", s: "abc", n: 3, suffix: ellipsis, want: "abc"},
		{name: "multibyte within limit", s: "привет", n: 6, suffix: ellipsis, want: "привет"},
		{name: "cut with suffix", s: "abcdef", n: 4, suffix: ellipsis, want: "abc…"},
		{name: "cut multibyte", s: "привет мир", n: 5, suffix: ellipsis, want: "прив…"},
		{name: "cut without suffix", s: "привет", n: 3, suffix: "", want: "при"},
		{name: "n equals suffix", s: "abcdef", n: 1, suffix: ellipsis, want: "…"},
		{name: "n smaller than suffix", s: "abcdef", n: 2, suffix: "[...]", want: "ab"},
		{name: "multi-rune suffix", s: "abcdefgh", n: 6, suffix: "[..]", want: "ab[..]"},
		{name: "whitespace kept", s: "  abc  ", n: 7, suffix: ellipsis, want: "  abc  "},
		{name: "trailing whitespace before cut", s: "ab   cdef", n: 5, suffix: ellipsis, want: "ab  …"},
		{name: "invalid bytes count as runes", s: "a\xffb\xffc", n: 4, suffix: "", want: "a\xffb\xff"},
		{name: "emoji", s: "😀😀😀", n: 2, suffix: "", want: "😀😀"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := corestrings.TruncateRunes(tc.s, tc.n, tc.suffix)
			require.Equal(t, tc.want, got)
			require.LessOrEqual(t, utf8.RuneCountInString(got), max(tc.n, 0))
		})
	}
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		n      int
		suffix string
		want   string
	}{
		{name: "empty", s: "", n: 3, suffix: ellipsis, want: ""},
		{name: "zero limit", s: "abc", n: 0, suffix: ellipsis, want: ""},
		{name: "negative limit", s: "abc", n: -4, suffix: ellipsis, want: ""},
		{name: "within limit", s: "abc", n: 3, suffix: ellipsis, want: "abc"},
		{name: "ascii cut with suffix", s: "abcdefgh", n: 6, suffix: ellipsis, want: "abc…"},
		{name: "ascii cut without suffix", s: "abcdefgh", n: 3, suffix: "", want: "abc"},
		// "п" is two bytes: a budget of 5 bytes holds two letters, not two and a half.
		{name: "no split two-byte", s: "привет", n: 5, suffix: "", want: "пр"},
		{name: "exact two-byte boundary", s: "привет", n: 4, suffix: "", want: "пр"},
		{name: "suffix budget", s: "привет", n: 8, suffix: ellipsis, want: "пр…"},
		{name: "no split four-byte", s: "😀😀", n: 7, suffix: "", want: "😀"},
		{name: "n smaller than suffix", s: "abcdef", n: 2, suffix: ellipsis, want: "ab"},
		{name: "n smaller than suffix multibyte", s: "привет", n: 2, suffix: ellipsis, want: "п"},
		{name: "n equals suffix", s: "abcdef", n: 3, suffix: ellipsis, want: "…"},
		{name: "whitespace kept", s: " a b ", n: 5, suffix: ellipsis, want: " a b "},
		{name: "invalid continuation bytes", s: "\x80\x80\x80\x80\x80", n: 3, suffix: "", want: "\x80\x80\x80"},
		{name: "truncated sequence then ascii", s: "a\xd0bcdef", n: 2, suffix: "", want: "a\xd0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := corestrings.TruncateBytes(tc.s, tc.n, tc.suffix)
			require.Equal(t, tc.want, got)
			require.LessOrEqual(t, len(got), max(tc.n, 0))
			if utf8.ValidString(tc.s) && utf8.ValidString(tc.suffix) {
				require.True(t, utf8.ValidString(got), "a valid input must stay valid")
			}
		})
	}
}

func TestTruncateBytesNeverSplitsValidInput(t *testing.T) {
	t.Parallel()

	s := strings.Repeat("aп€😀", 8)
	for n := range len(s) + 2 {
		for _, suffix := range []string{"", ellipsis, "..."} {
			got := corestrings.TruncateBytes(s, n, suffix)
			require.LessOrEqual(t, len(got), n)
			require.True(t, utf8.ValidString(got), "n=%d suffix=%q", n, suffix)
			require.True(t, strings.HasPrefix(s, strings.TrimSuffix(got, suffix)))
		}
	}
}
