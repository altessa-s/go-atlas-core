// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package optional

import (
	"bytes"
	"encoding/json"
	"errors"
)

// jsonNull is the canonical JSON encoding of None.
const jsonNull = "null"

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

// MarshalJSON encodes the Optional as JSON. Some(v) is encoded as v's
// JSON form; None is encoded as the JSON literal null.
//
// To omit None fields entirely with Go 1.25+, use json:",omitzero".
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.present {
		return []byte(jsonNull), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON decodes JSON into the Optional. The JSON literal null
// becomes None; any other input is decoded into T and stored as Some.
// An absent field never invokes UnmarshalJSON, so the zero value of
// Optional[T] (None) is the natural default.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(bytes.Trim(data, " \t\r\n")) == jsonNull {
		*o = None[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.value = v
	o.present = true
	return nil
}
