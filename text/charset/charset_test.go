// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package charset_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/text/charset"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const cyrillic = "код;название\n50;Московская область\n"

func encode(t *testing.T, enc encoding.Encoding, s string) []byte {
	t.Helper()
	out, err := enc.NewEncoder().Bytes([]byte(s))
	require.NoError(t, err)
	return out
}

func TestDecode(t *testing.T) {
	t.Parallel()

	cp1251 := encode(t, charmap.Windows1251, cyrillic)
	koi8 := encode(t, charmap.KOI8R, cyrillic)
	utf16 := encode(t, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), cyrillic)
	withBOM := append([]byte{0xef, 0xbb, 0xbf}, cyrillic...)

	cases := []struct {
		name        string
		raw         []byte
		contentType string
		fallback    encoding.Encoding
		want        string
	}{
		{"declared koi8-r", koi8, "text/csv; charset=koi8-r", charmap.Windows1251, cyrillic},
		{"declared label is case-insensitive and trimmed", koi8, `text/csv; charset=" KOI8-R "`, nil, cyrillic},
		{"declared alias cp1251", cp1251, "text/csv; charset=cp1251", nil, cyrillic},
		{"declared utf-16le", utf16, "text/plain; charset=utf-16le", nil, cyrillic},
		{"declared charset wins over valid utf-8", []byte("caf\xc3\xa9"), "text/plain; charset=windows-1252", nil, "cafÃ©"},
		{"undeclared valid utf-8 kept", []byte(cyrillic), "text/csv", charmap.Windows1251, cyrillic},
		{"undeclared utf-8 bom stripped", withBOM, "", charmap.Windows1251, cyrillic},
		{"undeclared legacy falls back", cp1251, "text/csv", charmap.Windows1251, cyrillic},
		{"empty content type falls back", cp1251, "", charmap.Windows1251, cyrillic},
		{"malformed content type is undeclared", cp1251, "text/csv; charset", charmap.Windows1251, cyrillic},
		{"mislabeled utf-8 falls back", cp1251, "text/csv; charset=utf-8", charmap.Windows1251, cyrillic},
		{"mislabeled utf8 alias falls back", cp1251, "text/csv; charset=UTF8", charmap.Windows1251, cyrillic},
		{"declared utf-8 valid kept", []byte(cyrillic), "text/csv; charset=utf-8", nil, cyrillic},
		{"declared utf-8 bom stripped", withBOM, "text/csv; charset=utf-8", nil, cyrillic},
		{"empty input", nil, "text/csv", charmap.Windows1251, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := charset.Decode(tc.raw, tc.contentType, tc.fallback)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()

	cp1251 := encode(t, charmap.Windows1251, cyrillic)
	cases := []struct {
		name        string
		raw         []byte
		contentType string
		fallback    encoding.Encoding
		want        error
	}{
		{"unknown label", []byte("abc"), "text/csv; charset=x-no-such-charset", charmap.Windows1251, charset.ErrUnknownCharset},
		{"invalid utf-8 without fallback", cp1251, "text/csv", nil, charset.ErrInvalidUTF8},
		{"mislabeled utf-8 without fallback", cp1251, "text/csv; charset=utf-8", nil, charset.ErrInvalidUTF8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := charset.Decode(tc.raw, tc.contentType, tc.fallback)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, got)
		})
	}
}

var errTransform = errors.New("transform failed")

// failingTransformer is a transform.Transformer that rejects every input.
type failingTransformer struct{ transform.NopResetter }

func (failingTransformer) Transform(_, _ []byte, _ bool) (nDst, nSrc int, err error) {
	return 0, 0, errTransform
}

// failingEncoding is an encoding.Encoding whose decoder always fails.
type failingEncoding struct{}

func (failingEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: failingTransformer{}}
}

func (failingEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: failingTransformer{}}
}

func TestDecodeFallbackFailure(t *testing.T) {
	t.Parallel()

	invalid := []byte{0xff, 0xfe, 'a'}
	got, err := charset.Decode(invalid, "text/plain", failingEncoding{})
	require.ErrorIs(t, err, errTransform, "the fallback decoder error is wrapped, not replaced")
	require.Nil(t, got)

	got, err = charset.Decode([]byte("valid"), "text/plain", failingEncoding{})
	require.NoError(t, err, "valid UTF-8 never reaches the fallback")
	require.Equal(t, []byte("valid"), got)
}
