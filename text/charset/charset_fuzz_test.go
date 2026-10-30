// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package charset_test

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/text/charset"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte("hello"), "text/plain; charset=utf-8")
	f.Add([]byte{0xef, 0xbb, 0xbf, 'a'}, "")
	f.Add([]byte{0xff, 0xfe, 0x41, 0}, "text/plain; charset=utf-16le")
	f.Add([]byte{0xff}, "text/plain; charset=windows-1251")
	f.Add([]byte{0xff}, "text/plain; charset")
	f.Add([]byte("hello"), "text/plain; charset=unknown")
	f.Fuzz(func(t *testing.T, raw []byte, contentType string) {
		original := bytes.Clone(raw)
		got, err := charset.Decode(raw, contentType, nil)
		require.Equal(t, original, raw)
		if err == nil {
			require.True(t, utf8.Valid(got))
		} else {
			require.Nil(t, got)
		}
		plain, err := charset.Decode(raw, "", nil)
		if utf8.Valid(raw) {
			require.NoError(t, err)
			require.Equal(t, bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), plain)
		} else {
			require.ErrorIs(t, err, charset.ErrInvalidUTF8)
		}
	})
}
