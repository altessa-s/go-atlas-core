// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package charset decodes downloaded bytes to UTF-8 when the source encoding may
// be declared, implicit UTF-8 or a known legacy single-byte charset.
//
// # Decision order
//
// [Decode] honors a charset parameter declared in the Content-Type first, using
// the WHATWG encoding index for labels and aliases. Without a declaration, valid
// UTF-8 is kept and a leading UTF-8 byte order mark is removed. Anything else is
// decoded with the caller's fallback encoding, typically a legacy code page such
// as windows-1251.
//
// # Mislabeled UTF-8
//
// A declared UTF-8 label is treated like no declaration: content that is not
// valid UTF-8 under that label is decoded with the fallback instead of producing
// replacement characters. Legacy servers commonly send single-byte files with a
// default "charset=utf-8".
//
// # Usage
//
//	body, err := charset.Decode(raw, resp.Header.Get("Content-Type"), charmap.Windows1251)
//	if errors.Is(err, charset.ErrUnknownCharset) {
//	    // the server declared a label nobody knows
//	}
package charset
