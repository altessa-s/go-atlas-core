// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package serializer provides a common [Serializer] interface for encoding and
// decoding arbitrary Go values. It is used by cache, uniq, and other packages
// that need format-agnostic value marshaling.
//
// The default implementation is [JSON], which delegates to
// [encoding/json/v2] with the options given to [NewJSON]; its zero value
// applies the v2 defaults.
//
// # Behavior change from encoding/json
//
// JSON used to delegate to encoding/json (v1). The v2 defaults are
// stricter: strings holding invalid UTF-8 fail to encode and decode,
// duplicate object member names fail to decode, struct fields match member
// names case-sensitively, nil slices and maps encode as [] and {} instead
// of null, and HTML characters are no longer escaped. Data written before
// the change may therefore be rejected. Options such as
// jsontext.AllowInvalidUTF8(true) relax individual rules, and
// NewJSON(jsonv1.DefaultOptionsV1()) with jsonv1 "encoding/json" restores
// the v1 behavior in full.
//
// Example:
//
//	s := serializer.NewJSON()
//	data, _ := s.Serialize(myStruct)
//	_ = s.Deserialize(data, &result)
package serializer
