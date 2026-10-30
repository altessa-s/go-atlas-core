// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package optional

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
)

// IsZero reports whether the Optional is None. It lets struct fields
// tagged with `bson:",omitempty"` (and other libraries that consult
// IsZero) skip None values on the wire.
func (o Optional[T]) IsZero() bool {
	return !o.present
}

// ErrBSONCodecRequired prevents silent BSON serialization of private fields.
// Configure the registry from data/mongo/bsoncodec to encode Optional values.
var ErrBSONCodecRequired = errors.New("optional: BSON requires data/mongo/bsoncodec.NewRegistry")

// MarshalBSONValue rejects unconfigured BSON encoders. The database adapter
// overrides this hook without coupling the value type to a database driver.
func (o Optional[T]) MarshalBSONValue() (byte, []byte, error) {
	return 0, nil, ErrBSONCodecRequired
}

// UnmarshalBSONValue rejects unconfigured BSON decoders without changing o.
func (o *Optional[T]) UnmarshalBSONValue(_ byte, _ []byte) error {
	return ErrBSONCodecRequired
}

// MarshalJSONTo implements [json.MarshalerTo]: None writes the JSON literal
// null, Some(v) writes v's JSON form. The value is encoded with the options
// of enc, so v follows the caller's codec: v2 semantics under [json.Marshal],
// v1 semantics under encoding/json, whose output is unchanged from the
// MarshalJSON method this replaced.
//
// To omit None fields entirely, use json:",omitzero".
func (o Optional[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !o.present {
		return enc.WriteToken(jsontext.Null)
	}
	// Pass the value, not &o.value: a non-addressable value keeps the v1
	// rule that pointer-receiver MarshalJSON methods on T are not called.
	return json.MarshalEncode(enc, o.value)
}

// UnmarshalJSONFrom implements [json.UnmarshalerFrom]: the JSON literal null
// becomes None; any other value is decoded into T with the options of dec
// and stored as Some. On error o is unchanged. An absent field never invokes
// UnmarshalJSONFrom, so the zero value of Optional[T] (None) is the natural
// default.
func (o *Optional[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == 'n' {
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		*o = None[T]()
		return nil
	}
	var v T
	if err := json.UnmarshalDecode(dec, &v); err != nil {
		return err
	}
	o.value = v
	o.present = true
	return nil
}
