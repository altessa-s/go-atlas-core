// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package maps_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

func TestMakeWeakRefNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  coremaps.WeakRef[testhelpers.User]
	}{
		{name: "nil object", ref: coremaps.MakeWeakRef[testhelpers.User](nil)},
		{name: "zero value", ref: coremaps.WeakRef[testhelpers.User]{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Nil(t, tc.ref.Value())
			require.False(t, tc.ref.IsAlive())
		})
	}
}

func TestWeakRefValue(t *testing.T) {
	t.Parallel()
	obj := &testhelpers.User{ID: 1, Name: "test"}
	ref := coremaps.MakeWeakRef(obj)

	require.True(t, ref.IsAlive())
	require.Same(t, obj, ref.Value())
	runtime.KeepAlive(obj)
}

func TestWeakRefCollected(t *testing.T) {
	t.Parallel()
	ref := coremaps.MakeWeakRef(&testhelpers.User{ID: 1, Name: "test"})

	// The object is unreachable; a few GC cycles are enough to collect it.
	for range 10 {
		if !ref.IsAlive() {
			break
		}
		runtime.GC()
	}
	require.False(t, ref.IsAlive())
	require.Nil(t, ref.Value())
}
