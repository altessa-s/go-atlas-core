// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package scheduler_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/scheduler"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

func TestParseSchedule(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		spec string
		want time.Time
	}{
		{"six fields with seconds", "30 * * * * *", from.Add(30 * time.Second)},
		{"every five minutes", "0 */5 * * * *", from.Add(5 * time.Minute)},
		{"every descriptor", "@every 90s", from.Add(90 * time.Second)},
		{"hourly descriptor", "@hourly", from.Add(time.Hour)},
		{"daily descriptor", "@daily", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{"cron tz prefix", "CRON_TZ=Asia/Tokyo 0 0 20 * * *", time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)},
		{"tz prefix", "TZ=UTC 0 0 12 * * *", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schedule, err := scheduler.ParseSchedule(tt.spec)
			require.NoError(t, err)
			require.True(t, tt.want.Equal(schedule.Next(from)), "next = %s, want %s", schedule.Next(from), tt.want)
		})
	}
}

func TestParseSchedule_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
	}{
		{"empty", ""},
		{"five fields without seconds", "*/5 * * * *"},
		{"seven fields", "0 0 0 * * * *"},
		{"out of range", "0 61 * * * *"},
		{"unknown descriptor", "@fortnightly"},
		{"bad every duration", "@every soon"},
		{"unknown zone", "CRON_TZ=Mars/Olympus 0 0 * * * *"},
		// Review finding R21-1: robfig/cron panics on a zone prefix without a space after it.
		{"cron tz without schedule", "CRON_TZ=UTC"},
		{"tz without schedule", "TZ=UTC"},
		{"tz tab separated", "TZ=UTC\t0 0 12 * * *"},
		{"tz with only a space", "TZ=UTC "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schedule, err := scheduler.ParseSchedule(tt.spec)
			require.Nil(t, schedule)
			require.ErrorIs(t, err, scheduler.ErrInvalidSchedule)
			require.ErrorIs(t, err, coreerrs.ErrInvalidArgument)
			require.ErrorContains(t, err, strconv.Quote(tt.spec))

			require.ErrorIs(t, scheduler.ValidateSchedule(tt.spec), scheduler.ErrInvalidSchedule)
		})
	}
}

func TestValidateSchedule(t *testing.T) {
	t.Parallel()

	require.NoError(t, scheduler.ValidateSchedule("@every 5m"))
	require.NoError(t, scheduler.ValidateSchedule("0 0 3 * * MON-FRI"))

	err := scheduler.ValidateSchedule("not a schedule")
	require.True(t, errors.Is(err, scheduler.ErrInvalidSchedule))
}
