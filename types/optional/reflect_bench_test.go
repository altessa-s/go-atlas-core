// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"reflect"
	"testing"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

// The reflection helpers run once per Optional field per encoded or decoded
// value in a reflection-driven codec, so they sit on the codec hot path.

func BenchmarkIsOptionalType(b *testing.B) {
	optType := reflect.TypeFor[optional.Optional[string]]()
	plainType := reflect.TypeFor[string]()

	b.Run("Optional", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !optional.IsOptionalType(optType) {
				b.Fatal("Optional not detected")
			}
		}
	})
	b.Run("Plain", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if optional.IsOptionalType(plainType) {
				b.Fatal("plain type detected as Optional")
			}
		}
	})
}

func BenchmarkGetReflect(b *testing.B) {
	type record struct {
		Name optional.Optional[string]
	}
	rec := record{Name: optional.Some("value")}

	b.Run("Addressable", func(b *testing.B) {
		field := reflect.ValueOf(&rec).Elem().Field(0)
		b.ReportAllocs()
		for b.Loop() {
			if v, ok := optional.GetReflect(field); !ok || v.String() != "value" {
				b.Fatal("unexpected value")
			}
		}
	})
	b.Run("NotAddressable", func(b *testing.B) {
		field := reflect.ValueOf(rec).Field(0)
		b.ReportAllocs()
		for b.Loop() {
			if v, ok := optional.GetReflect(field); !ok || v.String() != "value" {
				b.Fatal("unexpected value")
			}
		}
	})
}

func BenchmarkSomeReflect(b *testing.B) {
	optType := reflect.TypeFor[optional.Optional[string]]()
	value := reflect.ValueOf("value")
	b.ReportAllocs()
	for b.Loop() {
		if optional.SomeReflect(optType, value).IsZero() {
			b.Fatal("Some built as zero")
		}
	}
}
