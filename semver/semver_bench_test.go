// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package semver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/semver"
)

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		_, _ = semver.Parse("v1.2.3-alpha.1+build")
	}
}
func BenchmarkCompare(b *testing.B) {
	a, err := semver.Parse("1.2.3-alpha.1")
	require.NoError(b, err)
	other, err := semver.Parse("1.2.3-alpha.10")
	require.NoError(b, err)

	for b.Loop() {
		a.Compare(other)
	}
}
