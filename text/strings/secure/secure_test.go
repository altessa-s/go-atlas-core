// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package secure_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/text/strings/secure"
)

func TestNewString(t *testing.T) {
	ss := secure.NewString("secret")
	require.NotNil(t, ss)
	defer ss.Clear()

	require.False(t, ss.IsEmpty(), "IsEmpty() should be false")
	require.Equal(t, 6, ss.Len())
}

func TestString_String(t *testing.T) {
	ss := secure.NewString("secret")
	defer ss.Clear()

	require.Equal(t, "secret", ss.String())
}

func TestString_StringUnsafe(t *testing.T) {
	ss := secure.NewString("secret")
	defer ss.Clear()

	require.Equal(t, "secret", ss.StringUnsafe())
}

func TestString_Bytes(t *testing.T) {
	ss := secure.NewString("abc")
	defer ss.Clear()

	require.Equal(t, "abc", string(ss.Bytes()))
}

func TestString_BytesUnsafe(t *testing.T) {
	ss := secure.NewString("abc")
	defer ss.Clear()

	require.Equal(t, "abc", string(ss.BytesUnsafe()))
}

func TestString_Equal(t *testing.T) {
	a := secure.NewString("same")
	b := secure.NewString("same")
	c := secure.NewString("diff")
	defer a.Clear()
	defer b.Clear()
	defer c.Clear()

	require.True(t, a.Equal(b), "Equal() should be true for same content")
	require.False(t, a.Equal(c), "Equal() should be false for different content")
}

func TestString_Clear(t *testing.T) {
	ss := secure.NewString("secret")
	ss.Clear()

	require.True(t, ss.IsEmpty(), "IsEmpty() should be true after Clear()")
	require.Equal(t, 0, ss.Len())
}

func TestString_GoString(t *testing.T) {
	ss := secure.NewString("secret")
	defer ss.Clear()

	require.Equal(t, secure.GoStringRedacted, ss.GoString())
}

func TestString_Empty(t *testing.T) {
	ss := secure.NewString("")
	defer ss.Clear()

	require.True(t, ss.IsEmpty(), "IsEmpty() should be true for empty string")
	require.Equal(t, "", ss.String())
	require.Equal(t, "", ss.StringUnsafe())
	require.Nil(t, ss.Bytes())
	require.Nil(t, ss.BytesUnsafe())
}

func TestZeroBytes(t *testing.T) {
	b := []byte("secret")
	secure.ZeroBytes(b)
	for i, v := range b {
		require.Equal(t, byte(0), v, "ZeroBytes: byte[%d] should be 0", i)
	}
}

func TestString_Len(t *testing.T) {
	ss := secure.NewString("hello")
	require.Equal(t, 5, ss.Len())
}

func TestZeroString_Heap(t *testing.T) {
	t.Parallel()

	s := strings.Clone("secret")
	secure.ZeroString(s)
	require.Equal(t, strings.Repeat("\x00", len("secret")), s)
}

// TestZeroString_ConcurrentReadOnly pins that concurrent ZeroString calls on
// read-only literals recover their faults independently: SetPanicOnFault is
// per-goroutine, so no caller can disarm another's recovery.
func TestZeroString_ConcurrentReadOnly(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				secure.ZeroString("read-only literal")
			}
		})
	}
	wg.Wait()
}

func TestZeroString_Empty(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() { secure.ZeroString("") })
}

func TestNewString_HeapStorage(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", secure.DefaultSmallStringOptimizationThreshold+1)
	ss := secure.NewString(long)
	defer ss.Clear()

	require.False(t, ss.IsEmpty())
	require.Equal(t, len(long), ss.Len())
	require.Equal(t, long, ss.String())
	require.Equal(t, long, ss.StringUnsafe())
	require.Equal(t, long, string(ss.Bytes()))
	require.Equal(t, long, string(ss.BytesUnsafe()))
}

func TestString_Clear_HeapStorage(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("y", secure.DefaultSmallStringOptimizationThreshold+1)
	ss := secure.NewString(long)
	ss.Clear()

	require.True(t, ss.IsEmpty())
	require.Equal(t, 0, ss.Len())
	require.Equal(t, "", ss.String())
	require.Nil(t, ss.Bytes())
	require.Nil(t, ss.BytesUnsafe())
}

func TestString_Clear_Idempotent(t *testing.T) {
	t.Parallel()

	ss := secure.NewString("secret")
	ss.Clear()
	require.NotPanics(t, func() { ss.Clear() })
	require.True(t, ss.IsEmpty())
	require.Equal(t, 0, ss.Len())
}

func TestString_Equal_EmptyBoth(t *testing.T) {
	t.Parallel()

	a := secure.NewString("")
	b := secure.NewString("")
	defer a.Clear()
	defer b.Clear()

	require.True(t, a.Equal(b))
}

func TestString_Equal_DifferentLengths(t *testing.T) {
	t.Parallel()

	a := secure.NewString("short")
	b := secure.NewString("a-much-longer-value")
	defer a.Clear()
	defer b.Clear()

	require.False(t, a.Equal(b))
}

func TestString_Equal_HeapStorage(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("z", secure.DefaultSmallStringOptimizationThreshold+1)
	a := secure.NewString(long)
	b := secure.NewString(long)
	c := secure.NewString(strings.Repeat("w", secure.DefaultSmallStringOptimizationThreshold+1))
	defer a.Clear()
	defer b.Clear()
	defer c.Clear()

	require.True(t, a.Equal(b))
	require.False(t, a.Equal(c))
}

func TestString_GoString_Empty(t *testing.T) {
	t.Parallel()

	ss := secure.NewString("")
	defer ss.Clear()

	require.Equal(t, secure.GoStringEmpty, ss.GoString())
}

func TestZeroBytes_NoOp(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() { secure.ZeroBytes(nil) })
	require.NotPanics(t, func() { secure.ZeroBytes([]byte{}) })
}

func TestZeroBytes_LargeSlice(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Repeat("a", secure.OptimizedMemoryZeroingThreshold+1))
	secure.ZeroBytes(data)
	for i, v := range data {
		require.Equal(t, byte(0), v, "byte[%d] should be 0", i)
	}
}
