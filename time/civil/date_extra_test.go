// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package civil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/time/civil"
)

func TestMustNewPanicsOnInvalidDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		year  int
		month time.Month
		day   int
	}{
		{name: "February 30", year: 2026, month: time.February, day: 30},
		{name: "non-leap February 29", year: 2027, month: time.February, day: 29},
		{name: "month 13", year: 2026, month: 13, day: 1},
		{name: "year 0", year: 0, month: time.January, day: 1},
		{name: "year 10000", year: 10000, month: time.January, day: 1},
		{name: "day 0", year: 2026, month: time.January, day: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var recovered any
			func() {
				defer func() { recovered = recover() }()
				civil.MustNew(tc.year, tc.month, tc.day)
			}()
			err, ok := recovered.(error)
			require.True(t, ok, "MustNew must panic with an error, got %#v", recovered)
			require.ErrorIs(t, err, civil.ErrInvalidDate)
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	base := civil.MustNew(2026, time.June, 15)
	tests := []struct {
		name  string
		other civil.Date
		want  int
	}{
		{name: "equal", other: civil.MustNew(2026, time.June, 15), want: 0},
		{name: "earlier year wins over later month", other: civil.MustNew(2025, time.December, 31), want: 1},
		{name: "later year wins over earlier month", other: civil.MustNew(2027, time.January, 1), want: -1},
		{name: "earlier month wins over later day", other: civil.MustNew(2026, time.May, 31), want: 1},
		{name: "later month wins over earlier day", other: civil.MustNew(2026, time.July, 1), want: -1},
		{name: "earlier day", other: civil.MustNew(2026, time.June, 14), want: 1},
		{name: "later day", other: civil.MustNew(2026, time.June, 16), want: -1},
		{name: "zero date", other: civil.Date{}, want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, base.Compare(tc.other))
			require.Equal(t, -tc.want, tc.other.Compare(base), "Compare must be antisymmetric")
			require.Equal(t, tc.want < 0, base.Before(tc.other))
			require.Equal(t, tc.want > 0, base.After(tc.other))
		})
	}
}

// TestToday checks Today against the wall clock. Reading the clock on both
// sides of the call makes the assertion hold across a midnight rollover.
func TestToday(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+14", 14*60*60)
	before := civil.Of(time.Now().In(loc))
	got := civil.Today(loc)
	after := civil.Of(time.Now().In(loc))

	require.True(t, got == before || got == after, "Today(%s) = %s, want %s or %s", loc, got, before, after)
	require.True(t, got.Valid())
}
