// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package intern

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tests in this file are white-box: choosing strings that compete for one
// hot-cache slot needs the private slot hash, and the slot contents are not
// observable through the public API.

// collidingStrings returns two distinct strings that map to the same hot
// cache slot, and that slot.
func collidingStrings(tb testing.TB, si *Interner) (first, second string, slot uint32) {
	tb.Helper()
	seen := map[uint32]string{}
	for i := range 10_000 {
		s := "key-" + strconv.Itoa(i)
		slot := si.fastHash(s) % HotCacheSlots
		if prev, ok := seen[slot]; ok {
			return prev, s, slot
		}
		seen[slot] = s
	}
	tb.Fatal("no colliding strings found")
	return "", "", 0
}

func hotSlotValue(si *Interner, slot uint32) string {
	if p := si.hotCache[slot].Load(); p != nil {
		return *p
	}
	return ""
}

// TestPromoteToHotCache_ContestedSlot walks the promotion policy for a
// contested slot: the first string to reach HotCacheThreshold claims the
// slot, a competitor at the same count does not displace it, and the
// competitor takes over once a periodic re-check finds it more than twice as
// hot as the occupant.
func TestPromoteToHotCache_ContestedSlot(t *testing.T) {
	t.Parallel()

	si := New(1024)
	a, b, slot := collidingStrings(t, si)

	for range HotCacheThreshold {
		si.String(a)
	}
	require.Equal(t, a, hotSlotValue(si, slot), "reaching the threshold claims an empty slot")

	for range HotCacheThreshold {
		si.String(b)
	}
	require.Equal(t, a, hotSlotValue(si, slot), "an equally hot competitor must not displace the occupant")

	// b's next promotion check happens at PromotionCheckInterval accesses;
	// by then it has far more than twice a's access count.
	for range PromotionCheckInterval - HotCacheThreshold {
		si.String(b)
	}
	require.Equal(t, b, hotSlotValue(si, slot), "a much hotter competitor takes over the slot")

	si.ResetStats()
	require.Equal(t, a, si.String(a))
	require.Equal(t, b, si.String(b))
	stats := si.Stats()
	require.Equal(t, uint64(1), stats.HotHits, "only the slot occupant is served from the hot cache")
	require.Equal(t, uint64(1), stats.ColdHits, "the displaced string falls back to the cold cache")
}

func TestPromoteToHotCache_EvictedOccupantLosesSlot(t *testing.T) {
	t.Parallel()

	si := New(1024)
	a, b, slot := collidingStrings(t, si)

	for range HotCacheThreshold {
		si.String(a)
	}
	require.Equal(t, a, hotSlotValue(si, slot))

	// Simulate the occupant being evicted from the cold cache: the next
	// promotion into its slot takes it regardless of access counts.
	si.entries.Delete(a)
	si.promoteToHotCache(b, slot, HotCacheThreshold)

	require.Equal(t, b, hotSlotValue(si, slot))
}

func TestPromoteToHotCache_ColderCandidateKeepsOccupant(t *testing.T) {
	t.Parallel()

	si := New(1024)
	a, b, slot := collidingStrings(t, si)
	for range PromotionCheckInterval {
		si.String(a)
	}

	// Hot-cache hits do not count as accesses, so the occupant's count
	// stopped at the threshold when it was promoted.
	value, ok := si.entries.Load(a)
	require.True(t, ok)
	occupantCount := value.(*internEntry).accessCount.Load()
	require.Equal(t, int64(HotCacheThreshold), occupantCount)

	si.promoteToHotCache(b, slot, 2*occupantCount)
	require.Equal(t, a, hotSlotValue(si, slot), "exactly twice the occupant's count is not enough")

	si.promoteToHotCache(b, slot, 2*occupantCount+1)
	require.Equal(t, b, hotSlotValue(si, slot))
}
