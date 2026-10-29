// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package charset

import (
	"bytes"
	"errors"
	"mime"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// utf8BOM is the UTF-8 byte order mark stripped from valid UTF-8 input.
var utf8BOM = []byte{0xef, 0xbb, 0xbf}

var (
	// ErrUnknownCharset reports a declared charset label that the WHATWG
	// encoding index does not recognize.
	ErrUnknownCharset = errors.New("charset: unknown charset label")
	// ErrInvalidUTF8 reports input that is neither declared in a legacy charset
	// nor valid UTF-8, when no fallback encoding is configured.
	ErrInvalidUTF8 = errors.New("charset: input is not valid UTF-8")
)

// Decode returns raw as UTF-8, choosing the source encoding in this order:
//
//  1. A charset parameter declared in contentType wins. Labels are resolved
//     with the WHATWG encoding index (case-insensitive, aliases such as "cp1251"
//     included); an unrecognized label returns an error wrapping [ErrUnknownCharset].
//  2. Otherwise valid UTF-8 is kept, without a leading UTF-8 byte order mark.
//  3. Otherwise raw is decoded with fallback. A nil fallback returns an error
//     wrapping [ErrInvalidUTF8].
//
// A declared UTF-8 label (including aliases such as "utf8" or "unicode-1-1-utf-8")
// is treated like an undeclared charset: legacy servers often label single-byte
// content as UTF-8, so invalid UTF-8 under that label is decoded with fallback
// instead of being turned into replacement characters. A contentType that does
// not parse as a media type is treated as undeclared.
//
// The result may share memory with raw when no transcoding is needed.
func Decode(raw []byte, contentType string, fallback encoding.Encoding) ([]byte, error) {
	if label := declaredLabel(contentType); label != "" {
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, coreerrs.Wrapf(ErrUnknownCharset, "label %q", label)
		}
		if !isUTF8(enc) {
			return decode(raw, enc, label)
		}
	}
	if utf8.Valid(raw) {
		return bytes.TrimPrefix(raw, utf8BOM), nil
	}
	if fallback == nil {
		return nil, ErrInvalidUTF8
	}
	return decode(raw, fallback, "fallback")
}

func decode(raw []byte, enc encoding.Encoding, name string) ([]byte, error) {
	out, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return nil, coreerrs.Wrapf(err, "charset: decode %q", name)
	}
	return out, nil
}

// declaredLabel returns the trimmed charset parameter of contentType, or "".
func declaredLabel(contentType string) string {
	if contentType == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(params["charset"])
}

func isUTF8(enc encoding.Encoding) bool {
	name, err := htmlindex.Name(enc)
	return err == nil && name == "utf-8"
}
