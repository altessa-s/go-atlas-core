// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

func TestMarshalBSONValue_RequiresCodec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  optional.Optional[int]
	}{
		{name: "some", opt: optional.Some(7)},
		{name: "none", opt: optional.None[int]()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, data, err := tc.opt.MarshalBSONValue()
			require.ErrorIs(t, err, optional.ErrBSONCodecRequired)
			require.Zero(t, kind)
			require.Nil(t, data)
		})
	}
}

func TestUnmarshalBSONValue_LeavesValueUnchanged(t *testing.T) {
	t.Parallel()

	opt := optional.Some("kept")
	err := opt.UnmarshalBSONValue(0x02, []byte{1, 0, 0, 0, 0})
	require.ErrorIs(t, err, optional.ErrBSONCodecRequired)

	got, ok := opt.Get()
	require.True(t, ok)
	require.Equal(t, "kept", got)
}

func TestNoneReflect_PanicsOnNonOptional(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     reflect.Type
		wantMsg string
	}{
		{name: "nil type", typ: nil, wantMsg: "optional.NoneReflect: not an Optional[T] type: <nil>"},
		{name: "int", typ: reflect.TypeFor[int](), wantMsg: "optional.NoneReflect: not an Optional[T] type: int"},
		{
			name:    "foreign struct",
			typ:     reflect.TypeFor[struct{ A int }](),
			wantMsg: "optional.NoneReflect: not an Optional[T] type: struct { A int }",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, tc.wantMsg, func() { optional.NoneReflect(tc.typ) })
		})
	}
}

func TestInnerType_PanicsOnNilType(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "optional.InnerType: not an Optional[T] type: <nil>", func() {
		optional.InnerType(nil)
	})
}
