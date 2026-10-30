// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{"empty string", "", true},
		{"whitespace string", "   ", true},
		{"non-empty string", "test", false},
		{"nil ptr", (*string)(nil), true},
		{"ptr to empty", func() *string { s := ""; return &s }(), true},
		{"ptr to whitespace", func() *string { s := "  "; return &s }(), true},
		{"ptr to non-empty", func() *string { s := "test"; return &s }(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			switch v := tt.input.(type) {
			case string:
				got = corestrings.IsEmpty(v)
			case *string:
				got = corestrings.IsEmpty(v)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPtrConversions(t *testing.T) {
	t.Run("ToPtr", func(t *testing.T) {
		s := "test"
		p := corestrings.ToPtr(s)
		require.NotNil(t, p)
		require.Equal(t, "test", *p)
		require.Nil(t, corestrings.ToPtr(""))
		require.Nil(t, corestrings.ToPtr("  "))
	})

	t.Run("FromPtr", func(t *testing.T) {
		s := "test"
		require.Equal(t, "test", corestrings.FromPtr(&s))
		require.Equal(t, "", corestrings.FromPtr(nil))
	})
}

func TestTo(t *testing.T) {
	t.Run("Integers", func(t *testing.T) {
		require.Equal(t, 42, corestrings.ToInt("42"))
		require.Equal(t, int8(127), corestrings.To[int8]("127"))
		require.Equal(t, 0, corestrings.To[int]("invalid"))
	})

	t.Run("Floats", func(t *testing.T) {
		require.Equal(t, 3.14, corestrings.ToFloat64("3.14"))
		require.Equal(t, float32(0), corestrings.To[float32]("invalid"))
	})
}

// Named numeric types: To classifies them by their underlying kind.
type (
	namedInt   int16
	namedUint  uint32
	namedFloat float32
)

func TestToKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "int", got: corestrings.To[int]("-42"), want: -42},
		{name: "int plus sign", got: corestrings.To[int]("+7"), want: 7},
		{name: "int leading space", got: corestrings.To[int](" 7"), want: 0},
		{name: "int hex rejected", got: corestrings.To[int]("0x10"), want: 0},
		{name: "int exponent rejected", got: corestrings.To[int]("1e3"), want: 0},
		{name: "int empty", got: corestrings.To[int](""), want: 0},
		{name: "int8 max", got: corestrings.To[int8]("127"), want: int8(127)},
		{name: "int8 min", got: corestrings.To[int8]("-128"), want: int8(-128)},
		{name: "int8 overflow", got: corestrings.To[int8]("128"), want: int8(0)},
		{name: "int16 overflow", got: corestrings.To[int16]("32768"), want: int16(0)},
		{name: "int32 max", got: corestrings.To[int32]("2147483647"), want: int32(2147483647)},
		{name: "int32 overflow", got: corestrings.To[int32]("2147483648"), want: int32(0)},
		{name: "int64 min", got: corestrings.To[int64]("-9223372036854775808"), want: int64(-9223372036854775808)},
		{name: "int64 overflow", got: corestrings.To[int64]("9223372036854775808"), want: int64(0)},
		{name: "uint", got: corestrings.To[uint]("42"), want: uint(42)},
		{name: "uint negative", got: corestrings.To[uint]("-1"), want: uint(0)},
		{name: "uint plus sign rejected", got: corestrings.To[uint]("+1"), want: uint(0)},
		{name: "uint8 max", got: corestrings.To[uint8]("255"), want: uint8(255)},
		{name: "uint8 overflow", got: corestrings.To[uint8]("256"), want: uint8(0)},
		{name: "uint16 overflow", got: corestrings.To[uint16]("65536"), want: uint16(0)},
		{name: "uint32 overflow", got: corestrings.To[uint32]("4294967296"), want: uint32(0)},
		{name: "uint64 max", got: corestrings.To[uint64]("18446744073709551615"), want: uint64(18446744073709551615)},
		{name: "uint64 overflow", got: corestrings.To[uint64]("18446744073709551616"), want: uint64(0)},
		{name: "uintptr", got: corestrings.To[uintptr]("4096"), want: uintptr(4096)},
		{name: "uintptr negative", got: corestrings.To[uintptr]("-1"), want: uintptr(0)},
		{name: "float32", got: corestrings.To[float32]("3.14"), want: float32(3.14)},
		{name: "float32 overflow", got: corestrings.To[float32]("1e39"), want: float32(0)},
		{name: "float64 exponent", got: corestrings.To[float64]("1e3"), want: 1000.0},
		{name: "float64 hex", got: corestrings.To[float64]("0x1p-2"), want: 0.25},
		{name: "float64 overflow", got: corestrings.To[float64]("1e309"), want: 0.0},
		{name: "float64 invalid", got: corestrings.To[float64]("abc"), want: 0.0},
		{name: "named signed", got: corestrings.To[namedInt]("-300"), want: namedInt(-300)},
		{name: "named signed overflow", got: corestrings.To[namedInt]("40000"), want: namedInt(0)},
		{name: "named unsigned", got: corestrings.To[namedUint]("40000"), want: namedUint(40000)},
		{name: "named unsigned negative", got: corestrings.To[namedUint]("-1"), want: namedUint(0)},
		{name: "named float", got: corestrings.To[namedFloat]("2.5"), want: namedFloat(2.5)},
		{name: "ToUint16", got: corestrings.ToUint16("65535"), want: uint16(65535)},
		{name: "ToInt32", got: corestrings.ToInt32("-5"), want: int32(-5)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		opts     corestrings.JoinOptions
		want     string
	}{
		{
			name:     "basic join",
			elements: []string{"a", "b", "c"},
			opts:     corestrings.JoinOptions{Separator: ","},
			want:     "a,b,c",
		},
		{
			name:     "skip empty",
			elements: []string{"a", "", "b"},
			opts:     corestrings.JoinOptions{Separator: ",", SkipEmpty: true},
			want:     "a,b",
		},
		{
			name:     "with prefix suffix",
			elements: []string{"a", "b"},
			opts:     corestrings.JoinOptions{Separator: ",", Prefix: "(", Suffix: ")"},
			want:     "(a,b)",
		},
		{
			name:     "empty result",
			elements: []string{},
			opts:     corestrings.JoinOptions{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, corestrings.Join(tt.elements, tt.opts))
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name string
		s    string
		opts corestrings.SplitOptions
		want []string
	}{
		{
			name: "basic split",
			s:    "a,b,c",
			opts: corestrings.SplitOptions{Separator: ","},
			want: []string{"a", "b", "c"},
		},
		{
			name: "skip empty parts",
			s:    "a,,b",
			opts: corestrings.SplitOptions{Separator: ",", SkipEmpty: true},
			want: []string{"a", "b"},
		},
		{
			name: "trim space",
			s:    "a , b ",
			opts: corestrings.SplitOptions{Separator: ",", TrimSpace: true},
			want: []string{"a", "b"},
		},
		{
			name: "case insensitive",
			s:    "aANDb",
			opts: corestrings.SplitOptions{Separator: "and", CaseSensitive: false},
			want: []string{"a", "b"},
		},
		{
			name: "max splits",
			s:    "a,b,c",
			opts: corestrings.SplitOptions{Separator: ",", MaxSplits: 1},
			want: []string{"a", "b,c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := corestrings.Split(tt.s, tt.opts)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestContains(t *testing.T) {
	t.Run("Case Insensitive", func(t *testing.T) {
		res := corestrings.Contains("Hello World", "hello", corestrings.ContainsOptions{CaseSensitive: false})
		require.True(t, res.Found, "Should find 'hello' in 'Hello World' (case-insensitive)")
	})

	t.Run("Whole Words", func(t *testing.T) {
		require.False(t, corestrings.Contains("hello world", "hell", corestrings.ContainsOptions{MatchWholeWords: true}).Found, "Should NOT find 'hell' in 'hello world' as whole word")
		require.True(t, corestrings.Contains("hello world", "hello", corestrings.ContainsOptions{MatchWholeWords: true}).Found, "Should find 'hello' in 'hello world' as whole word")
	})

	// Regression: isWholeWordMatch used to index s[pos-1] / s[endPos]
	// directly, converting UTF-8 continuation bytes (0x80-0xBF) to their
	// Latin-1 codepoint values, none of which are IsLetter/IsDigit. The
	// boundary check then wrongly reported matches inside a multi-byte
	// rune as whole-word hits.
	t.Run("Whole Words NonASCII", func(t *testing.T) {
		// "héllo": h, é (0xC3 0xA9), l, l, o — searching for "llo" lands
		// at byte offset 3, preceded by the continuation byte 0xA9.
		// Before the fix this returned Found=true.
		require.False(t, corestrings.Contains("héllo", "llo", corestrings.ContainsOptions{MatchWholeWords: true}).Found, "Should NOT find 'llo' in 'héllo' as whole word (suffix inside a word)")
		// Symmetric case: "holé" — searching for "hol" at offset 0 is
		// followed by the leading byte of é. The decoded next rune is
		// a letter, so the match must be rejected.
		require.False(t, corestrings.Contains("holé", "hol", corestrings.ContainsOptions{MatchWholeWords: true}).Found, "Should NOT find 'hol' in 'holé' as whole word (prefix inside a word)")
		// Positive case: a full non-ASCII word surrounded by spaces must
		// still be detected.
		require.True(t, corestrings.Contains(" café ", "café", corestrings.ContainsOptions{MatchWholeWords: true}).Found, "Should find 'café' in ' café ' as whole word")
	})

	t.Run("Count", func(t *testing.T) {
		res := corestrings.Contains("test test", "test", corestrings.ContainsOptions{Count: true})
		require.Equal(t, 2, res.Count)
		require.Equal(t, []int{0, 5}, res.Positions)
	})

	// Regression: the whole-word boundary check used to index the ORIGINAL
	// string with offsets computed on the LOWERCASED copy. When case folding
	// changes byte lengths ("İ" U+0130, 2 bytes, lowers to "i̇",
	// 3 bytes) the offsets shift and a legitimate match was rejected.
	t.Run("Whole Words CaseFold LengthChange", func(t *testing.T) {
		res := corestrings.Contains("İstanbul word", "word", corestrings.ContainsOptions{
			CaseSensitive:   false,
			MatchWholeWords: true,
		})
		require.True(t, res.Found, "Should find 'word' after a length-changing case fold")
	})

	// Contract: Count and Positions are populated only when opts.Count is
	// true — including the empty-substring fast path.
	t.Run("Empty Substr Honors Count Option", func(t *testing.T) {
		res := corestrings.Contains("abc", "", corestrings.ContainsOptions{})
		require.True(t, res.Found)
		require.Zero(t, res.Count)
		require.Nil(t, res.Positions)

		res = corestrings.Contains("abc", "", corestrings.ContainsOptions{Count: true})
		require.True(t, res.Found)
		require.Equal(t, 1, res.Count)
		require.Equal(t, []int{0}, res.Positions)
	})
}

func TestSafeComparisons(t *testing.T) {
	require.True(t, corestrings.SubstringMatch("Hello World", "world"), "SubstringMatch failed")
}

func TestUnsafe(t *testing.T) {
	t.Run("ToBytesUnsafe", func(t *testing.T) {
		s := "hello"
		b := corestrings.ToBytesUnsafe(s)
		require.Equal(t, s, string(b))
		// Verify zero-copy by checking address (careful with GC moving things, but basic check)
		// We expect b to point to s data.
		sData := unsafe.StringData(s)
		bData := unsafe.SliceData(b)
		require.Equal(t, sData, bData, "ToBytesUnsafe did not return zero-copy slice")
	})

	t.Run("FromBytesUnsafe", func(t *testing.T) {
		b := []byte("hello")
		s := corestrings.FromBytesUnsafe(b)
		require.Equal(t, "hello", s)
	})

	t.Run("StringEqualsUnsafe", func(t *testing.T) {
		require.True(t, corestrings.StringEqualsUnsafe("a", "a"), "StringEqualsUnsafe failed for equal")
		require.False(t, corestrings.StringEqualsUnsafe("a", "b"), "StringEqualsUnsafe passed for different")
	})
}

func TestConcat(t *testing.T) {
	require.Equal(t, "abc", corestrings.Concat("a", "b", "c"))
	require.Equal(t, "ab", corestrings.ConcatUnsafe("a", "b"))
}

func TestCase(t *testing.T) {
	require.True(t, corestrings.IsLowercase("abc"))
	require.False(t, corestrings.IsLowercase("Abc"))
	require.True(t, corestrings.IsUppercase("ABC"))
	require.False(t, corestrings.IsUppercase("Abc"))

	require.True(t, corestrings.IsLowercaseUnsafe("abc"))
	require.False(t, corestrings.IsLowercaseUnsafe("Abc"))
	require.True(t, corestrings.IsUppercaseUnsafe("ABC"))
	require.False(t, corestrings.IsUppercaseUnsafe("Abc"))
}

func TestTrim(t *testing.T) {
	t.Run("IsTrimmed", func(t *testing.T) {
		require.True(t, corestrings.IsTrimmed("abc"), "abc should be trimmed")
		require.False(t, corestrings.IsTrimmed(" abc"), "' abc' should not be trimmed")
		require.True(t, corestrings.IsTrimmed(""), "empty string should be trimmed")
	})

	t.Run("IsTrimmedUnsafe", func(t *testing.T) {
		require.True(t, corestrings.IsTrimmedUnsafe("abc"))
		require.False(t, corestrings.IsTrimmedUnsafe(" abc"))
	})

	t.Run("TrimSuffixFast", func(t *testing.T) {
		require.Equal(t, "file", corestrings.TrimSuffixFast("file.go", ".go"))
		require.Equal(t, "file.go", corestrings.TrimSuffixFast("file.go", ".txt"))
	})
}
