// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bits

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBitWidth_PredeclaredTypes pins the width of every predeclared integer
// type through the exported functions whose result is the width itself:
// CountTrailingZeros and CountLeadingZeros of zero, and the highest valid
// bit position.
func TestBitWidth_PredeclaredTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width uint64
		ctz   func() uint64
		clz   func() uint64
		isSet func(pos uint64) bool
	}{
		{"int8", 8, func() uint64 { return CountTrailingZeros(int8(0)) }, func() uint64 { return CountLeadingZeros(int8(0)) },
			func(p uint64) bool { return IsBitSet(int8(-1), p) }},
		{"uint8", 8, func() uint64 { return CountTrailingZeros(uint8(0)) }, func() uint64 { return CountLeadingZeros(uint8(0)) },
			func(p uint64) bool { return IsBitSet(uint8(0xff), p) }},
		{"int16", 16, func() uint64 { return CountTrailingZeros(int16(0)) }, func() uint64 { return CountLeadingZeros(int16(0)) },
			func(p uint64) bool { return IsBitSet(int16(-1), p) }},
		{"uint16", 16, func() uint64 { return CountTrailingZeros(uint16(0)) }, func() uint64 { return CountLeadingZeros(uint16(0)) },
			func(p uint64) bool { return IsBitSet(^uint16(0), p) }},
		{"int32", 32, func() uint64 { return CountTrailingZeros(int32(0)) }, func() uint64 { return CountLeadingZeros(int32(0)) },
			func(p uint64) bool { return IsBitSet(int32(-1), p) }},
		{"uint32", 32, func() uint64 { return CountTrailingZeros(uint32(0)) }, func() uint64 { return CountLeadingZeros(uint32(0)) },
			func(p uint64) bool { return IsBitSet(^uint32(0), p) }},
		{"int64", 64, func() uint64 { return CountTrailingZeros(int64(0)) }, func() uint64 { return CountLeadingZeros(int64(0)) },
			func(p uint64) bool { return IsBitSet(int64(-1), p) }},
		{"uint64", 64, func() uint64 { return CountTrailingZeros(uint64(0)) }, func() uint64 { return CountLeadingZeros(uint64(0)) },
			func(p uint64) bool { return IsBitSet(^uint64(0), p) }},
		{"int", strconv.IntSize, func() uint64 { return CountTrailingZeros(0) }, func() uint64 { return CountLeadingZeros(0) },
			func(p uint64) bool { return IsBitSet(-1, p) }},
		{"uint", strconv.IntSize, func() uint64 { return CountTrailingZeros(uint(0)) }, func() uint64 { return CountLeadingZeros(uint(0)) },
			func(p uint64) bool { return IsBitSet(^uint(0), p) }},
		{"uintptr", strconv.IntSize, func() uint64 { return CountTrailingZeros(uintptr(0)) },
			func() uint64 { return CountLeadingZeros(uintptr(0)) }, func(p uint64) bool { return IsBitSet(^uintptr(0), p) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.width, tc.ctz())
			require.Equal(t, tc.width, tc.clz())
			require.True(t, tc.isSet(tc.width-1), "highest bit position must be valid")
			require.False(t, tc.isSet(tc.width), "position equal to the width must be rejected")
		})
	}
}
