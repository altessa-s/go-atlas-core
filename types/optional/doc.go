// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package optional provides a generic Optional[T] type that holds
// either a value of type T or nothing. It makes the "value may be
// absent" intent explicit at the type level, for places where Go's
// idiomatic (T, bool) pair or *T is awkward — struct fields, channel
// elements, slice values, map values.
//
// Internally Optional carries (value, present) by value. Some(v)
// does not allocate, the type is comparable when T is comparable,
// and the zero value of Optional[T] is a valid None. All exported
// constructors and methods are pure and safe for concurrent use.
//
// # Usage
//
//	// Struct field where nil-pointer would conflate "absent" with
//	// "present but zero".
//	type User struct {
//	    Name     string
//	    Nickname optional.Optional[string]
//	}
//
//	u := User{Name: "Ada", Nickname: optional.Some("")}
//	if nick, ok := u.Nickname.Get(); ok {
//	    fmt.Println("nickname is set, value:", nick) // distinguishes Some("") from None
//	}
//
//	// Bridge from the idiomatic (value, ok) tuple.
//	v, ok := m[k]
//	opt := optional.Of(v, ok)
//
// # When NOT to use
//
// For ordinary lookups keep the (value, ok) idiom: `v, ok := m[k]`,
// `v, ok := <-ch`. Wrapping every such pair in Optional adds noise
// without removing any.
//
// For optional pointer fields where nil already unambiguously means
// "absent", *T is fine. Reach for Optional[T] when T's zero value is
// itself a valid Some and you need to distinguish it from None, or
// when you want the type signature to advertise optionality.
//
// For "value or error" use core/types/result.Result instead.
//
// # Serialization
//
// JSON support is built in: Some(v) encodes as v, None as null. Use the
// json:",omitzero" field tag to omit None with Go 1.25+.
//
// JSON is implemented by [Optional.MarshalJSONTo] and
// [Optional.UnmarshalJSONFrom], the encoding/json/v2 interfaces, which
// encoding/json also calls. They encode and decode T with the options of the
// calling codec, so under encoding/json the wire format is unchanged from
// the former MarshalJSON method, and under encoding/json/v2 Some(v) behaves
// like a bare v (v2 rejects invalid UTF-8 and encodes a nil slice as []).
// Optional does not implement json.Marshaler or json.Unmarshaler.
//
// BSON support is provided by data/mongo/bsoncodec.NewRegistry. Configure the
// Mongo client or standalone encoder/decoder with that registry. Unconfigured
// BSON calls fail with ErrBSONCodecRequired instead of losing private fields.
// None encodes as BSON null (or is omitted with omitempty); Some(zero) preserves
// its presence. Some(nil) is indistinguishable from None on the BSON wire.
// This package has no external production dependencies.
package optional
