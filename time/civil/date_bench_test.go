// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package civil_test

import (
	"testing"
	"time"

	"github.com/altessa-s/go-atlas-core/time/civil"
)

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := civil.Parse("2026-10-09"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkString(b *testing.B) {
	d := civil.MustNew(2026, time.October, 9)
	b.ReportAllocs()
	for b.Loop() {
		_ = d.String()
	}
}

func BenchmarkAddMonthsClamped(b *testing.B) {
	d := civil.MustNew(2026, time.January, 31)
	b.ReportAllocs()
	for b.Loop() {
		_ = d.AddMonthsClamped(13, 31)
	}
}

func BenchmarkDaysSince(b *testing.B) {
	d, o := civil.MustNew(2026, time.October, 9), civil.MustNew(2000, time.January, 1)
	b.ReportAllocs()
	for b.Loop() {
		_ = d.DaysSince(o)
	}
}

// BenchmarkOf measures truncating a zoned instant to its calendar date.
func BenchmarkOf(b *testing.B) {
	t := time.Date(2026, time.October, 9, 23, 59, 59, 0, time.FixedZone("UTC+3", 3*60*60))
	b.ReportAllocs()
	for b.Loop() {
		if !civil.Of(t).Valid() {
			b.Fatal("invalid date")
		}
	}
}

// BenchmarkAddDays measures date arithmetic across month and year
// boundaries.
func BenchmarkAddDays(b *testing.B) {
	d := civil.MustNew(2026, time.December, 30)
	b.ReportAllocs()
	for b.Loop() {
		if d.AddDays(400).Before(d) {
			b.Fatal("date moved backwards")
		}
	}
}

// BenchmarkWeekday measures the day-of-week computation used by schedules.
func BenchmarkWeekday(b *testing.B) {
	d := civil.MustNew(2026, time.October, 9)
	b.ReportAllocs()
	for b.Loop() {
		if d.Weekday() != time.Friday {
			b.Fatal("unexpected weekday")
		}
	}
}

// BenchmarkIn measures converting a date to midnight in a location.
func BenchmarkIn(b *testing.B) {
	d := civil.MustNew(2026, time.October, 9)
	loc := time.FixedZone("UTC+3", 3*60*60)
	b.ReportAllocs()
	for b.Loop() {
		if d.In(loc).IsZero() {
			b.Fatal("zero time")
		}
	}
}

// BenchmarkTextRoundTrip measures the MarshalText/UnmarshalText pair that
// text-based encoders (YAML, form values, map keys) go through.
func BenchmarkTextRoundTrip(b *testing.B) {
	d := civil.MustNew(2026, time.October, 9)
	b.ReportAllocs()
	for b.Loop() {
		text, err := d.MarshalText()
		if err != nil {
			b.Fatal(err)
		}
		var got civil.Date
		if err := got.UnmarshalText(text); err != nil {
			b.Fatal(err)
		}
		if got != d {
			b.Fatal("round trip mismatch")
		}
	}
}
