// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

// collectGarbage runs several GC cycles and gives the cleanup goroutine time to
// run the cleanups they queue.
func collectGarbage(tb testing.TB) {
	tb.Helper()
	for range 10 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
}

// Re-setting the value a key already holds must not register another cleanup.
// Serial: testing.AllocsPerRun counts process-wide allocations and refuses to
// run in a parallel test.
func TestWeakMap_SetSameLiveValueAllocatesNothing(t *testing.T) {
	m := coremaps.NewWeakMap[string, testhelpers.User]()
	val := &testhelpers.User{ID: 1}
	m.Set("k", val)

	allocs := testing.AllocsPerRun(1000, func() { m.Set("k", val) })
	require.Zero(t, allocs)
	runtime.KeepAlive(val)
}

// Every replaced value must have its cleanup stopped: cleanups of live values
// otherwise pile up in the runtime, one per Set. The runtime keeps cleanup
// records off the Go heap and accounts them in MemStats.OtherSys, reusing the
// records Stop frees, so the growth of OtherSys bounds the number of live
// records. Serial: it reads process-wide memory statistics.
func TestWeakMap_ReplaceStopsPreviousCleanup(t *testing.T) {
	m := coremaps.NewWeakMap[int, testhelpers.User]()
	a, b := &testhelpers.User{ID: 1}, &testhelpers.User{ID: 2}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	const sets = 64_000
	for i := range sets / 2 {
		m.Set(i%16, a)
		m.Set(i%16, b)
	}
	runtime.ReadMemStats(&after)

	// A leaked record per Set would add several megabytes; allow 1 MiB of
	// unrelated runtime growth.
	grown := int64(after.OtherSys) - int64(before.OtherSys)
	require.Less(t, grown, int64(1<<20), "OtherSys grew by %d bytes over %d Set calls", grown, sets)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
}

// The cleanup of a replaced value must never delete the entry that replaced it.
func TestWeakMap_CollectingReplacedValueKeepsNewEntry(t *testing.T) {
	t.Parallel()

	m := coremaps.NewWeakMap[string, testhelpers.User]()
	m.Set("replaced", &testhelpers.User{ID: 1})
	m.Set("deleted", &testhelpers.User{ID: 2})
	m.Delete("deleted")

	kept, rekeyed := &testhelpers.User{ID: 3}, &testhelpers.User{ID: 4}
	m.Set("replaced", kept)
	m.Set("deleted", rekeyed)

	collectGarbage(t)

	got, ok := m.Get("replaced")
	require.True(t, ok)
	require.Same(t, kept, got)
	got, ok = m.Get("deleted")
	require.True(t, ok)
	require.Same(t, rekeyed, got)
	require.Equal(t, 2, m.Len())
	runtime.KeepAlive(kept)
	runtime.KeepAlive(rekeyed)
}

// A value cached under several keys keeps one cleanup per key, so collecting it
// removes every entry.
func TestWeakMap_ValueUnderSeveralKeysIsRemovedFromAll(t *testing.T) {
	t.Parallel()

	m := coremaps.NewWeakMap[string, testhelpers.User]()
	func() {
		shared := &testhelpers.User{ID: 1}
		m.Set("a", shared)
		m.Set("b", shared)
		m.Set("a", shared)
	}()

	collectGarbage(t)

	require.Zero(t, m.Len())
}
