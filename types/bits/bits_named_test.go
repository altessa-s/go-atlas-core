// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bits_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/bits"
	"github.com/altessa-s/go-atlas-core/types/constraints"
)

type (
	namedInt     int
	namedInt8    int8
	namedInt16   int16
	namedInt32   int32
	namedInt64   int64
	namedUint    uint
	namedUint8   uint8
	namedUint16  uint16
	namedUint32  uint32
	namedUint64  uint64
	namedUintptr uintptr
	// namedOfNamed is defined over another named type; its underlying type is still uint8.
	namedOfNamed namedUint8
)

// requireWidth asserts that every width-dependent function treats T as width bits wide.
func requireWidth[T constraints.Integer](t *testing.T, width uint64) {
	t.Helper()

	top := T(1) << (width - 1)
	require.Equal(t, width, bits.CountTrailingZeros(T(0)), "CountTrailingZeros(0)")
	require.Equal(t, width, bits.CountLeadingZeros(T(0)), "CountLeadingZeros(0)")
	require.Equal(t, width-1, bits.CountLeadingZeros(T(1)), "CountLeadingZeros(1)")
	require.Equal(t, width, bits.FindLastSet(top), "FindLastSet(top bit)")
	require.Equal(t, T(3), bits.RotateLeft(top|1, 1), "RotateLeft(top|1, 1)")
	require.Equal(t, top|1, bits.RotateRight(T(3), 1), "RotateRight(3, 1)")
	require.Equal(t, top, bits.ReverseBits(T(1)), "ReverseBits(1)")
	require.Equal(t, ^T(0), bits.CreateMask[T](width), "CreateMask(width)")
	require.Zero(t, bits.CreateMask[T](width+1), "CreateMask(width+1)")
	require.True(t, bits.IsBitSet(^T(0), width-1), "IsBitSet(top position)")
	require.False(t, bits.IsBitSet(^T(0), width), "IsBitSet(width)")
	require.Equal(t, top, bits.SetBit(T(0), width-1), "SetBit(top position)")
	require.Zero(t, bits.SetBit(T(0), width), "SetBit(width)")
	require.Equal(t, top, bits.SetBits(T(0), width-1, 1), "SetBits(top position, 1)")
	require.Zero(t, bits.SetBits(T(0), width-1, 2), "SetBits past the width")
	require.False(t, bits.TestBits(^T(0), width-1, 2), "TestBits past the width")
	require.Zero(t, bits.FindNextSet(^T(0), width-1), "FindNextSet(top position)")
	require.Equal(t, top, bits.SignExtend(top, width), "SignExtend(top, width)")
	require.Equal(t, top, bits.SignExtend(top, width+1), "SignExtend(top, width+1)")
}

func TestWidth_NamedIntegerTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"int", func(t *testing.T) { requireWidth[namedInt](t, strconv.IntSize) }},
		{"int8", func(t *testing.T) { requireWidth[namedInt8](t, 8) }},
		{"int16", func(t *testing.T) { requireWidth[namedInt16](t, 16) }},
		{"int32", func(t *testing.T) { requireWidth[namedInt32](t, 32) }},
		{"int64", func(t *testing.T) { requireWidth[namedInt64](t, 64) }},
		{"uint", func(t *testing.T) { requireWidth[namedUint](t, strconv.IntSize) }},
		{"uint8", func(t *testing.T) { requireWidth[namedUint8](t, 8) }},
		{"uint16", func(t *testing.T) { requireWidth[namedUint16](t, 16) }},
		{"uint32", func(t *testing.T) { requireWidth[namedUint32](t, 32) }},
		{"uint64", func(t *testing.T) { requireWidth[namedUint64](t, 64) }},
		{"uintptr", func(t *testing.T) { requireWidth[namedUintptr](t, strconv.IntSize) }},
		{"named of named uint8", func(t *testing.T) { requireWidth[namedOfNamed](t, 8) }},
		{"predeclared int8", func(t *testing.T) { requireWidth[int8](t, 8) }},
		{"predeclared uint", func(t *testing.T) { requireWidth[uint](t, strconv.IntSize) }},
		{"predeclared uintptr", func(t *testing.T) { requireWidth[uintptr](t, strconv.IntSize) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t)
		})
	}
}

// The values from the defect report: a named byte must behave like uint8.
func TestNamedByte(t *testing.T) {
	t.Parallel()

	type myByte uint8
	require.Equal(t, uint64(7), bits.CountLeadingZeros(myByte(1)))
	require.Equal(t, uint64(8), bits.CountTrailingZeros(myByte(0)))
	require.Equal(t, myByte(3), bits.RotateLeft(myByte(0x81), 1))
	require.Equal(t, myByte(128), bits.ReverseBits(myByte(1)))
}
