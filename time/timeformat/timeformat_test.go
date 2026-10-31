// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package timeformat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatParse_RoundTripInstant(t *testing.T) {
	t.Parallel()

	// Use a fixed instant to avoid flakiness.
	in := time.Unix(1700000000, 123456789).UTC()

	formats := []Format{RFC3339, RFC3339Nano, Unix, UnixMilli, UnixMicro, UnixNano}
	for _, f := range formats {
		t.Run(f.String(), func(t *testing.T) {
			t.Parallel()
			out := f.Format(in)
			parsed, err := f.Parse(out)
			require.NoError(t, err, "Parse (out=%q)", out)

			// Round-trip preserves only the precision of the chosen format.
			switch f {
			case RFC3339, Unix:
				require.Equal(t, in.Unix(), parsed.Unix())
			case RFC3339Nano, UnixNano:
				require.Equal(t, in.UnixNano(), parsed.UnixNano())
			case UnixMilli:
				require.Equal(t, in.UnixMilli(), parsed.UnixMilli())
			case UnixMicro:
				require.Equal(t, in.UnixMicro(), parsed.UnixMicro())
			default:
				require.Failf(t, "unhandled format", "%q", f)
			}
		})
	}
}

func TestFormatTime_Types(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)

	_, ok := FormatTime(now, RFC3339).(string)
	require.True(t, ok, "RFC3339 should format to string")
	_, ok = FormatTime(now, Unix).(int64)
	require.True(t, ok, "Unix should format to int64")
	_, ok = FormatTime(now, UnixMilli).(int64)
	require.True(t, ok, "UnixMilli should format to int64")
	_, ok = FormatTime(now, UnixMicro).(int64)
	require.True(t, ok, "UnixMicro should format to int64")
	_, ok = FormatTime(now, UnixNano).(int64)
	require.True(t, ok, "UnixNano should format to int64")
}

func TestFormatDuration_UnixMicroIsNanoseconds(t *testing.T) {
	t.Parallel()

	d := 1500 * time.Microsecond // 1_500_000 ns
	got := FormatDuration(d, UnixMicro)
	ns, ok := got.(int64)
	require.True(t, ok, "expected int64 for UnixMicro duration, got %T", got)
	require.Equal(t, d.Nanoseconds(), ns)
}

func TestFormatTime_Values(t *testing.T) {
	t.Parallel()

	in := time.Date(2023, 11, 14, 22, 13, 20, 123456789, time.UTC)
	tests := []struct {
		format Format
		want   any
	}{
		{RFC3339, "2023-11-14T22:13:20Z"},
		{RFC3339Nano, "2023-11-14T22:13:20.123456789Z"},
		{Unix, int64(1700000000)},
		{UnixMilli, int64(1700000000123)},
		{UnixMicro, int64(1700000000123456)},
		{UnixNano, int64(1700000000123456789)},
		{Format("bogus"), "2023-11-14T22:13:20Z"},
		{Format(""), "2023-11-14T22:13:20Z"},
	}
	for _, tc := range tests {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, FormatTime(in, tc.format))
		})
	}
}

func TestFormatDuration_AllFormats(t *testing.T) {
	t.Parallel()

	d := 90*time.Second + 250*time.Millisecond
	tests := []struct {
		format Format
		want   any
	}{
		{Unix, d.Nanoseconds()},
		{UnixMilli, d.Nanoseconds()},
		{UnixMicro, d.Nanoseconds()},
		{UnixNano, d.Nanoseconds()},
		{RFC3339, d},
		{RFC3339Nano, d},
		{Format("bogus"), d},
	}
	for _, tc := range tests {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, FormatDuration(d, tc.format))
		})
	}
}
