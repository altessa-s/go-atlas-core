// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

func BenchmarkMarshalJSONTo_Some(b *testing.B) {
	opt := optional.Some("hello")
	var (
		raw []byte
		err error
	)
	for b.Loop() {
		raw, err = json.Marshal(opt)
	}
	_, _ = raw, err
}

func BenchmarkMarshalJSONTo_None(b *testing.B) {
	opt := optional.None[string]()
	var (
		raw []byte
		err error
	)
	for b.Loop() {
		raw, err = json.Marshal(opt)
	}
	_, _ = raw, err
}

func BenchmarkUnmarshalJSONFrom_Some(b *testing.B) {
	in := []byte(`"hello"`)
	var opt optional.Optional[string]
	for b.Loop() {
		if err := json.Unmarshal(in, &opt); err != nil {
			require.NoError(b, err)
		}
	}
}

func BenchmarkUnmarshalJSONFrom_Null(b *testing.B) {
	in := []byte("null")
	var opt optional.Optional[string]
	for b.Loop() {
		if err := json.Unmarshal(in, &opt); err != nil {
			require.NoError(b, err)
		}
	}
}
