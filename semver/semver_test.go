// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package semver_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/semver"
)

func TestParse(t *testing.T) {
	t.Parallel()

	got, err := semver.Parse("v12.0.3-rc.1.00a+build.001")
	require.NoError(t, err)
	require.Equal(t, semver.Version{Major: "12", Minor: "0", Patch: "3", Prerelease: []string{"rc", "1", "00a"}, Build: "build.001"}, got)

	for _, s := range []string{
		"", "1", "1.2", "1.2.3.4", "01.0.0", "1.00.0", "1.0.00",
		"1.0.0-01", "1.0.0+", "1.0.0-", "1.0.0-x..y", "1.0.0+a.",
		"1.0.0+a+b", "1.0.0-α", "1.0.0+a_b", "V1.0.0", " 1.0.0", "1.0.0\n",
	} {
		t.Run(s, func(t *testing.T) {
			t.Parallel()

			_, err := semver.Parse(s)
			require.ErrorIs(t, err, semver.ErrInvalid)
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1.0.0-rc1", "1.0.0", -1}, {"1.0.0-rc.2", "1.0.0-rc.10", -1}, {"1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1}, {"1.0.0+build.123", "v1.0.0+other", 0}, {"1.0.0-alpha", "1.0.0-beta", -1},
		{"9.99.99", "10.0.0", -1}, {"1.9.99", "1.10.0", -1}, {"1.0.9", "1.0.10", -1},
		{"123456789012345678900.0.0", "123456789012345678901.0.0", -1},
		{"0.0.0-99999999999999999999", "0.0.0-100000000000000000000", -1},
	} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			t.Parallel()

			a, err := semver.Parse(tc.a)
			require.NoError(t, err)
			b, err := semver.Parse(tc.b)
			require.NoError(t, err)
			require.Equal(t, tc.want, a.Compare(b))
			require.Equal(t, -tc.want, b.Compare(a))
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add("v1.2.3-beta.1+meta")
	f.Add("01.0.0")
	f.Fuzz(func(t *testing.T, s string) {
		v, err := semver.Parse(s)
		if err != nil {
			return
		}

		raw := v.Major + "." + v.Minor + "." + v.Patch
		if len(v.Prerelease) > 0 {
			raw += "-" + strings.Join(v.Prerelease, ".")
		}

		if v.Build != "" {
			raw += "+" + v.Build
		}

		require.Equal(t, strings.TrimPrefix(s, "v"), raw)
		other, err := semver.Parse(raw)
		require.NoError(t, err)
		require.Zero(t, v.Compare(other))
	})
}
