// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors_test

import (
	"errors"
	"fmt"
	"testing"

	coreerrors "github.com/altessa-s/go-atlas-core/errors"
)

func BenchmarkWalk(b *testing.B) {
	leaf := errors.New("leaf")

	tree := errors.Join(fmt.Errorf("wrapped: %w", leaf), leaf)
	for b.Loop() {
		coreerrors.Walk(tree, 4, func(error) bool { return true })
	}
}
