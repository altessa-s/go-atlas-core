// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

func TestSubstringMatch_NotFound(t *testing.T) {
	require.False(t, corestrings.SubstringMatch("Hello", "xyz"), "should not match")
}

func TestSubstringMatch_ShorterString(t *testing.T) {
	require.False(t, corestrings.SubstringMatch("Hi", "Hello"), "shorter string should not match longer substr")
}

func TestSubstringMatch_Empty(t *testing.T) {
	require.True(t, corestrings.SubstringMatch("anything", ""), "empty substr should always match")
}

func TestToPtr_EmptyString(t *testing.T) {
	require.Nil(t, corestrings.ToPtr(""))
}

func TestToPtr_NonEmpty(t *testing.T) {
	p := corestrings.ToPtr("hello")
	require.NotNil(t, p)
	require.Equal(t, "hello", *p)
}

func TestIsTrimmed(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{" hello", false},
		{"hello ", false},
		{" hello ", false},
		{"", true},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, corestrings.IsTrimmed(tt.input))
	}
}

func TestIsTrimmedUnsafe(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{" hello", false},
		{"hello ", false},
		{"", true},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, corestrings.IsTrimmedUnsafe(tt.input))
	}
}

func TestTrimSuffixFast(t *testing.T) {
	tests := []struct {
		s, suffix, want string
	}{
		{"hello.go", ".go", "hello"},
		{"hello", ".go", "hello"},
		{".go", ".go", ""},
		{"", ".go", ""},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, corestrings.TrimSuffixFast(tt.s, tt.suffix))
	}
}

func TestStringEqualsUnsafe(t *testing.T) {
	require.True(t, corestrings.StringEqualsUnsafe("hello", "hello"), "same string should be equal")
	require.False(t, corestrings.StringEqualsUnsafe("hi", "hello"), "different length should not be equal")
	require.False(t, corestrings.StringEqualsUnsafe("abc", "xyz"), "different strings should not be equal")
}

func TestSplitSeq_CaseInsensitive(t *testing.T) {
	opts := corestrings.SplitOptions{Separator: "SEP", CaseSensitive: false}
	count := 0
	for range corestrings.SplitSeq("asepbSEPc", opts) {
		count++
	}
	require.Equal(t, 3, count)
}

func TestGetStringBuilder(t *testing.T) {
	sb := corestrings.GetStringBuilder()
	require.NotNil(t, sb)
	sb.WriteString("test")
	corestrings.PutStringBuilder(sb)
}

func TestToScreamingSnakeCase_Complex(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"camelCase", "CAMEL_CASE"},
		{"HTTPServer", "HTTP_SERVER"},
		{"simpleTest", "SIMPLE_TEST"},
		{"", ""},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, corestrings.ToScreamingSnakeCase(tt.input))
	}
}

func TestToPtr_StringPointer(t *testing.T) {
	t.Parallel()

	word := "hello"
	blank := " \t\n"
	empty := ""

	tests := []struct {
		name string
		in   *string
		want *string
	}{
		{name: "nil pointer", in: nil, want: nil},
		{name: "empty value", in: &empty, want: nil},
		{name: "whitespace value", in: &blank, want: nil},
		{name: "non-empty value returns the same pointer", in: &word, want: &word},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := corestrings.ToPtr(tc.in)
			if tc.want == nil {
				require.Nil(t, got)
				return
			}
			require.Same(t, tc.want, got)
		})
	}
}

func TestToPtr_StringReturnsFreshPointer(t *testing.T) {
	t.Parallel()

	a := corestrings.ToPtr(" padded ")
	b := corestrings.ToPtr(" padded ")
	require.NotNil(t, a)
	require.Equal(t, " padded ", *a, "the value is not trimmed")
	require.NotSame(t, a, b)
}

func TestUnsafeConversions_Empty(t *testing.T) {
	t.Parallel()

	require.Nil(t, corestrings.ToBytesUnsafe(""))
	require.Empty(t, corestrings.FromBytesUnsafe(nil))
	require.Empty(t, corestrings.FromBytesUnsafe([]byte{}))
	require.Empty(t, corestrings.FromBytesUnsafe(make([]byte, 0, 16)))
}

func TestConcat_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{name: "no parts", parts: nil, want: ""},
		{name: "one part", parts: []string{"solo"}, want: "solo"},
		{name: "all empty", parts: []string{"", "", ""}, want: ""},
		{name: "some empty", parts: []string{"a", "", "b"}, want: "ab"},
		{name: "multibyte", parts: []string{"при", "вет", "!"}, want: "привет!"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, corestrings.ConcatUnsafe(tc.parts...))
			require.Equal(t, tc.want, corestrings.Concat(tc.parts...))
		})
	}
}

// TestConcatUnsafe_DoesNotAliasInputs guards the safety contract: the result
// owns a fresh buffer, so it does not alias any input part.
func TestConcatUnsafe_DoesNotAliasInputs(t *testing.T) {
	t.Parallel()

	src := []byte("abc")
	got := corestrings.ConcatUnsafe(corestrings.FromBytesUnsafe(src), "def")
	src[0] = 'X'
	require.Equal(t, "abcdef", got)
}

func TestPutStringBuilder_NilAndReset(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() { corestrings.PutStringBuilder(nil) })

	sb := corestrings.GetStringBuilder()
	sb.WriteString("leftover")
	corestrings.PutStringBuilder(sb)
	require.Zero(t, sb.Len(), "PutStringBuilder resets the builder")

	next := corestrings.GetStringBuilder()
	require.Zero(t, next.Len(), "a pooled builder is handed out empty")
	corestrings.PutStringBuilder(next)
}

func TestJoin_SkipEmptyAllEmpty(t *testing.T) {
	t.Parallel()

	got := corestrings.Join([]string{"", "", ""}, corestrings.JoinOptions{
		Separator: ",", SkipEmpty: true, Prefix: "[", Suffix: "]",
	})
	require.Empty(t, got, "prefix and suffix are not added to an empty result")
}

func TestSplit_CaseInsensitiveInvalidUTF8Separator(t *testing.T) {
	t.Parallel()

	got := corestrings.Split("a\xffB\xffc", corestrings.SplitOptions{Separator: "\xff", MaxSplits: -1})
	require.Equal(t, []string{"a", "B", "c"}, got)
}
