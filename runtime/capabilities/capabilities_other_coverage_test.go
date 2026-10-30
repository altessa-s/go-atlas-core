// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build !linux

package capabilities_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/capabilities"
)

// Every mutating entry point is unsupported off Linux.
func TestMutators_UnsupportedOffLinux(t *testing.T) {
	t.Parallel()

	calls := map[string]func() error{
		"DropAll":         capabilities.DropAll,
		"DropAllExcept":   func() error { return capabilities.DropAllExcept(capabilities.Cap(0)) },
		"Apply":           func() error { return capabilities.Apply(capabilities.Sets{}) },
		"BoundingDrop":    func() error { return capabilities.BoundingDrop(capabilities.Cap(0)) },
		"AmbientClearAll": capabilities.AmbientClearAll,
		"AmbientRaise":    func() error { return capabilities.AmbientRaise(capabilities.Cap(0)) },
		"AmbientLower":    func() error { return capabilities.AmbientLower(capabilities.Cap(0)) },
	}
	for name, call := range calls {
		require.ErrorIs(t, call(), capabilities.ErrUnsupported, name)
	}
}
