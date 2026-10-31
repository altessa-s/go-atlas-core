// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package intern_test

import (
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/text/strings/intern"
	"github.com/altessa-s/go-atlas-core/time/clock"
)

func TestNew(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.NotNil(t, interner)
	require.Equal(t, int64(100), interner.MaxSize())
	require.True(t, interner.IsEmpty(), "new interner should be empty")
}

func TestInterner_String(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)

	s1 := interner.String("hello")
	s2 := interner.String("hello")
	require.Equal(t, s1, s2, "String() should return same string for same input")
	require.False(t, interner.IsEmpty(), "interner should not be empty after String()")
}

func TestInterner_UpperString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "HELLO", interner.UpperString("hello"))
}

func TestInterner_LowerString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "hello", interner.LowerString("HELLO"))
}

func TestInterner_TrimString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "hello", interner.TrimString("  hello  "))
}

func TestInterner_PrefixString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "hello-world", interner.PrefixString("world", "hello-"))
}

func TestInterner_SuffixString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "hello-world", interner.SuffixString("hello", "-world"))
}

func TestInterner_WrapString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "[hello]", interner.WrapString("hello", "[", "]"))
}

func TestInterner_CleanPathString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "/a/b", interner.CleanPathString("/a//b/"))
}

func TestInterner_StringSlice(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	input := []string{"a", "b", "c"}
	got := interner.StringSlice(input)
	require.Equal(t, input, got)
}

func TestInterner_StringMap(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	input := map[string]string{"k": "v"}
	got := interner.StringMap(input)
	require.Equal(t, "v", got["k"])
}

func TestInterner_FormatString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "hello world", interner.FormatString("hello %s", "world"))
}

func TestInterner_JoinString(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Equal(t, "a,b,c", interner.JoinString([]string{"a", "b", "c"}, ","))
}

func TestInterner_JoinWith(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	got := interner.JoinWith([]string{"a", "b"}, func(parts []string) string {
		return strings.Join(parts, "-")
	})
	require.Equal(t, "a-b", got)
}

func TestInterner_JoinEmptyPartsDoesNotIntern(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	called := false

	require.Empty(t, interner.JoinString(nil, ","))
	require.Empty(t, interner.JoinWith([]string{}, func([]string) string {
		called = true
		return "unexpected"
	}))
	require.False(t, called, "the joiner is not called for empty parts")
	require.True(t, interner.IsEmpty())
	require.Equal(t, intern.Stats{MaxSize: 100}, interner.Stats(), "the empty string bypasses every cache")
}

func TestInterner_JoinResultIsCanonical(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	joined := interner.JoinString([]string{"a", "b"}, ",")
	viaJoiner := interner.JoinWith([]string{"a", "b"}, func(parts []string) string { return strings.Join(parts, ",") })

	require.Equal(t, "a,b", viaJoiner)
	require.Equal(t, int64(1), interner.Size(), "both joins intern the same entry")
	require.Same(t, unsafe.StringData(joined), unsafe.StringData(viaJoiner))
}

// TestInterner_String_CacheTiers follows one string through the three lookup
// tiers: a miss creates it, cold-cache hits count accesses until it reaches
// HotCacheThreshold and is promoted, and later lookups hit the hot cache.
func TestInterner_String_CacheTiers(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	for range intern.HotCacheThreshold {
		require.Equal(t, "tiered", interner.String("tiered"))
	}

	stats := interner.Stats()
	require.Equal(t, uint64(1), stats.Misses)
	require.Equal(t, uint64(intern.HotCacheThreshold-1), stats.ColdHits)
	require.Zero(t, stats.HotHits, "promotion happens on the threshold access, not before")

	for range 3 {
		require.Equal(t, "tiered", interner.String("tiered"))
	}
	stats = interner.Stats()
	require.Equal(t, uint64(3), stats.HotHits)
	require.Equal(t, uint64(intern.HotCacheThreshold-1), stats.ColdHits)
	require.Equal(t, int64(1), interner.Size(), "the hot cache does not add entries")
}

func TestInterner_String_EmptyIsNotInterned(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	require.Empty(t, interner.String(""))
	require.True(t, interner.IsEmpty())
	require.Zero(t, interner.Stats().Misses)
}

func TestWithClock_NilKeepsDefault(t *testing.T) {
	t.Parallel()

	interner := intern.New(10, intern.WithClock(nil))
	require.Equal(t, "x", interner.String("x"))
	require.Equal(t, int64(1), interner.Size())
}

func TestInterner_SizeAndLoadFactor(t *testing.T) {
	t.Parallel()

	interner := intern.New(10)

	require.Equal(t, int64(0), interner.Size())
	require.Equal(t, float64(0), interner.LoadFactor())

	interner.String("a")
	require.GreaterOrEqual(t, interner.Size(), int64(1))
	if interner.IsFull() {
		require.GreaterOrEqual(t, interner.Size(), interner.MaxSize())
	}
}

func TestInterner_Reset(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	interner.String("hello")
	interner.Reset()
	require.True(t, interner.IsEmpty(), "IsEmpty() should be true after Reset()")
}

// The tests of the process-wide singleton below stay serial: they share the
// package-level default interner (intern.Default), which intern.ResetDefault
// clears, so a concurrent test could repopulate or clear it mid-assertion.

func TestDefault(t *testing.T) {
	t.Cleanup(intern.ResetDefault)

	g := intern.Default()
	require.NotNil(t, g)
}

func TestResetDefault(t *testing.T) {
	t.Cleanup(intern.ResetDefault)

	intern.String("leak-check")
	g := intern.Default()
	require.False(t, g.IsEmpty(), "default interner should not be empty after String")

	intern.ResetDefault()
	require.True(t, g.IsEmpty(), "default interner should be empty after ResetDefault")
}

func TestInterner_Stats(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)

	// Initially all counters are zero.
	stats := interner.Stats()
	require.Equal(t, uint64(0), stats.HotHits)
	require.Equal(t, uint64(0), stats.ColdHits)
	require.Equal(t, uint64(0), stats.Misses)
	require.Equal(t, uint64(0), stats.Evictions)
	require.Equal(t, uint64(0), stats.TotalLookups())
	require.Equal(t, float64(0), stats.HitRate())

	// First lookup is a miss.
	interner.String("hello")
	stats = interner.Stats()
	require.Equal(t, uint64(1), stats.Misses)

	// Second lookup for the same string is a cold hit.
	interner.String("hello")
	stats = interner.Stats()
	require.GreaterOrEqual(t, stats.ColdHits, uint64(1))

	// Hit rate should be > 0.
	require.Greater(t, stats.HitRate(), float64(0))
	require.Equal(t, stats.HotHits+stats.ColdHits+stats.Misses, stats.TotalLookups())

	// MaxSize and CurrentSize should be populated.
	require.Equal(t, int64(100), stats.MaxSize)
	require.GreaterOrEqual(t, stats.CurrentSize, int64(1))
}

func TestInterner_ResetStats(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	interner.String("foo")
	interner.String("foo")

	interner.ResetStats()
	stats := interner.Stats()
	require.Equal(t, uint64(0), stats.HotHits)
	require.Equal(t, uint64(0), stats.ColdHits)
	require.Equal(t, uint64(0), stats.Misses)
	require.Equal(t, uint64(0), stats.Evictions)

	// Cache entries should still exist.
	require.False(t, interner.IsEmpty(), "interner should not be empty after ResetStats")
}

func TestInterner_StatsAfterReset(t *testing.T) {
	t.Parallel()

	interner := intern.New(100)
	interner.String("bar")
	interner.Reset()

	stats := interner.Stats()
	require.Equal(t, uint64(0), stats.HotHits)
	require.Equal(t, uint64(0), stats.ColdHits)
	require.Equal(t, uint64(0), stats.Misses)
}

func TestStats_HitRate(t *testing.T) {
	t.Parallel()

	s := intern.Stats{HotHits: 3, ColdHits: 5, Misses: 2}
	require.Equal(t, 0.8, s.HitRate())
	require.Equal(t, 0.3, s.HotHitRate())
	require.Equal(t, uint64(10), s.TotalLookups())
}

func TestStats_HitRateZero(t *testing.T) {
	t.Parallel()

	s := intern.Stats{}
	require.Equal(t, float64(0), s.HitRate())
	require.Equal(t, float64(0), s.HotHitRate())
}

// TestDefaultFunctions stays serial: the package-level wrappers intern into the
// shared default interner (intern.Default), and its intern.ResetDefault cleanup
// would race with TestResetDefault's emptiness assertions.
func TestDefaultFunctions(t *testing.T) {
	t.Cleanup(intern.ResetDefault)

	require.Equal(t, "test", intern.String("test"))
	require.Equal(t, "test", intern.LowerString("TEST"))
	require.Equal(t, "TEST", intern.UpperString("test"))
	require.Equal(t, "x", intern.TrimString("  x  "))
	require.Equal(t, "/a/b", intern.CleanPathString("/a//b/"))
	require.Equal(t, "a-b", intern.PrefixString("b", "a-"))
	require.Equal(t, "a-b", intern.SuffixString("a", "-b"))
	require.Equal(t, "(x)", intern.WrapString("x", "(", ")"))
	require.Equal(t, []string{"a"}, intern.StringSlice([]string{"a"}))
	require.Equal(t, "v", intern.StringMap(map[string]string{"k": "v"})["k"])
	require.Equal(t, "42", intern.FormatString("%d", 42))
	require.Equal(t, "a-b", intern.JoinString([]string{"a", "b"}, "-"))
	require.Equal(t, "a+b", intern.JoinWith([]string{"a", "b"}, func(p []string) string { return strings.Join(p, "+") }))
}

// TestInterner_WithClock_EvictsLeastRecentOnTie pins that access timestamps
// come from the injected clock: with equal access counts, eviction removes the
// entries the fake clock stamped earliest and keeps the most recent one.
func TestInterner_WithClock_EvictsLeastRecentOnTie(t *testing.T) {
	t.Parallel()

	fake := clock.NewFake(time.Unix(1_000, 0))
	interner := intern.New(2, intern.WithClock(fake))

	interner.String("oldest")
	fake.Advance(time.Minute)
	interner.String("middle")
	fake.Advance(time.Minute)
	interner.String("newest") // exceeds maxSize and triggers background eviction

	require.Eventually(t, func() bool { return interner.Size() == 1 }, time.Second, time.Millisecond)

	interner.ResetStats()
	interner.String("newest")
	stats := interner.Stats()
	require.Equal(t, uint64(1), stats.ColdHits, "most recently stamped entry must survive eviction")
	require.Equal(t, uint64(0), stats.Misses)
}
