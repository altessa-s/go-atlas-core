// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package bits provides generic bit manipulation functions for all integer types.
// It covers single-bit operations, range operations, counting, rotation, and masking.
// The bit width of a type is the size of its underlying type, so a named type
// such as type myByte uint8 is 8 bits wide, and int, uint and uintptr follow the
// target platform.
//
// Example:
//
//	value := uint8(5) // 00000101
//	bits.IsBitSet(value, 0)  // true
//	bits.SetBit(value, 1)    // 00000111 (7)
//	bits.CountSetBits(value) // 2
package bits
