// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package strings provides utilities for string manipulation and conversion:
// validation, case transformation, truncation, join/split, pooling, and
// zero-copy conversions.
//
// All utility functions are pure and thread-safe. Secret handling lives in
// [github.com/altessa-s/go-atlas-core/text/strings/secure] and string interning
// in [github.com/altessa-s/go-atlas-core/text/strings/intern].
//
// # Options Pattern
//
// Functions with multiple configuration options use Options structs:
//   - [JoinOptions] for [Join]: separator, prefix, suffix, skip empty.
//   - [SplitOptions] for [Split]: separator, max splits, trim, case sensitivity.
//   - [ContainsOptions] for [Contains]: case sensitivity, whole words, counting.
//
// # Usage
//
//	// Validation and conversion
//	strings.IsEmpty("  ")              // true
//	ptr := strings.ToPtr("hello")      // *string
//
//	// Case transformation
//	strings.ToSnakeCase("HelloWorld")  // "hello_world"
//	strings.ToCamelCase("hello_world") // "helloWorld"
//
//	// Truncation (the suffix counts toward the limit; runes are never split)
//	strings.TruncateRunes("привет мир", 5, "…") // "прив…"
//	strings.TruncateBytes("привет", 5, "")      // "пр"
//
// # Performance
//
//   - Zero-copy operations: ToBytesUnsafe/FromBytesUnsafe avoid allocations.
//   - Object pooling: pooled string slices and builders reduce GC pressure.
package strings
