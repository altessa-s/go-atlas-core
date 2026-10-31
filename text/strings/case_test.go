// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

// TestIsTrimmed_Unicode pins the non-ASCII branches of IsTrimmed and
// IsTrimmedUnsafe: both must treat Unicode whitespace at either end as
// untrimmed and other multibyte runes as trimmed, and agree with each other.
func TestIsTrimmed_Unicode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "ascii word", input: "hello", want: true},
		{name: "single ascii space", input: " ", want: false},
		{name: "tab at start", input: "\thello", want: false},
		{name: "newline at end", input: "hello\n", want: false},
		{name: "cyrillic word", input: "привет", want: true},
		{name: "cyrillic start ascii end", input: "привет!", want: true},
		{name: "ascii start cyrillic end", input: "!привет", want: true},
		{name: "no-break space at start", input: " hello", want: false},
		{name: "no-break space at end", input: "hello ", want: false},
		{name: "ideographic space at end", input: "日本　", want: false},
		{name: "em space at start before multibyte", input: " 日本", want: false},
		{name: "only unicode space", input: " ", want: false},
		{name: "emoji both ends", input: "😀 x 😀", want: true},
		{name: "multibyte start ascii space end", input: "日本 ", want: false},
		{name: "ascii space start multibyte end", input: " 日本", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, corestrings.IsTrimmed(tc.input), "IsTrimmed(%q)", tc.input)
			require.Equal(t, tc.want, corestrings.IsTrimmedUnsafe(tc.input), "IsTrimmedUnsafe(%q)", tc.input)
		})
	}
}

// TestCaseUnsafe_FallsBackForNonASCII checks that the byte-level fast path
// of IsUppercaseUnsafe and IsLowercaseUnsafe hands non-ASCII input to the
// Unicode-aware checks and agrees with them.
func TestCaseUnsafe_FallsBackForNonASCII(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantUpper bool
		wantLower bool
	}{
		{name: "empty", input: "", wantUpper: true, wantLower: true},
		{name: "upper cyrillic", input: "ПРИВЕТ", wantUpper: true, wantLower: false},
		{name: "lower cyrillic", input: "привет", wantUpper: false, wantLower: true},
		{name: "mixed cyrillic", input: "Привет", wantUpper: false, wantLower: false},
		{name: "ascii prefix then non-ascii", input: "ABCÄ", wantUpper: true, wantLower: false},
		{name: "digits and symbols", input: "123-!", wantUpper: true, wantLower: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.wantUpper, corestrings.IsUppercaseUnsafe(tc.input))
			require.Equal(t, tc.wantUpper, corestrings.IsUppercase(tc.input))
			require.Equal(t, tc.wantLower, corestrings.IsLowercaseUnsafe(tc.input))
			require.Equal(t, tc.wantLower, corestrings.IsLowercase(tc.input))
		})
	}
}
