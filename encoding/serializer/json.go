// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package serializer

import (
	"encoding/json/v2"
)

// JSON is a [Serializer] implementation backed by [encoding/json/v2].
// It delegates to [json.Marshal] and [json.Unmarshal] with the options
// given to [NewJSON], so all v2 rules apply (struct tags, custom
// marshalers, etc.).
//
// The v2 defaults are stricter than those of encoding/json (v1): invalid
// UTF-8 and duplicate object member names are rejected, struct fields
// match member names case-sensitively, nil slices and maps encode as []
// and {} instead of null, and HTML characters are not escaped. Pass
// jsontext.AllowInvalidUTF8(true), jsontext.AllowDuplicateNames(true) or
// other [json.Options] to [NewJSON] to relax individual rules, or
// encoding/json.DefaultOptionsV1() to restore the v1 behavior in full.
//
// JSON is immutable and safe for concurrent use from multiple goroutines.
// A zero value is ready to use and applies the v2 defaults.
//
// Example:
//
//	s := serializer.NewJSON(json.Deterministic(true))
//	bytes, err := s.Serialize(&user)
//	var loaded User
//	err = s.Deserialize(bytes, &loaded)
type JSON struct {
	opts []json.Options
}

// NewJSON returns a JSON serializer that passes opts to every Marshal and
// Unmarshal call. Later options override earlier ones, as in
// [json.JoinOptions]. Marshal-only options are ignored by Deserialize and
// unmarshal-only options by Serialize.
func NewJSON(opts ...json.Options) *JSON {
	if len(opts) == 0 {
		return &JSON{}
	}
	return &JSON{opts: []json.Options{json.JoinOptions(opts...)}}
}

// Serialize encodes data as JSON using [json.Marshal] and returns the
// resulting bytes. It returns an error if data contains values that cannot
// be represented in JSON (e.g., channels, complex numbers, cycles, or
// strings holding invalid UTF-8 unless the options allow it).
func (s *JSON) Serialize(data any) ([]byte, error) {
	return json.Marshal(data, s.opts...)
}

// Deserialize decodes the JSON-encoded bytes in d into out using
// [json.Unmarshal]. The out parameter must be a non-nil pointer. It
// returns an error if d is not valid JSON — under the default options this
// includes invalid UTF-8 and duplicate member names — or cannot be mapped
// to the target type.
func (s *JSON) Deserialize(d []byte, out any) error {
	return json.Unmarshal(d, out, s.opts...)
}

var _ Serializer = (*JSON)(nil)
