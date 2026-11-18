// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package semver

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Version holds the components of a parsed semantic version. Numeric components
// are decimal strings so precedence is independent of machine integer size.
type Version struct {
	Major, Minor, Patch string
	Prerelease          []string
	Build               string
}

const (
	number      = `(0|[1-9][0-9]*)`
	identifiers = `([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)`
)

// ErrInvalid is wrapped by every error [Parse] returns.
var ErrInvalid = errors.New("invalid semantic version")

var pattern = regexp.MustCompile(`^v?` + number + `\.` + number + `\.` + number + `(?:-` + identifiers + `)?(?:\+` + identifiers + `)?$`)

// Parse accepts a SemVer 2.0.0 version with an optional lowercase v prefix.
func Parse(s string) (Version, error) {
	m := pattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("%w %q", ErrInvalid, s)
	}

	v := Version{Major: m[1], Minor: m[2], Patch: m[3], Build: m[5]}
	if m[4] != "" {
		v.Prerelease = strings.Split(m[4], ".")
		for _, id := range v.Prerelease {
			if len(id) > 1 && id[0] == '0' && numeric(id) {
				return Version{}, fmt.Errorf("%w: leading zero in prerelease identifier %q", ErrInvalid, id)
			}
		}
	}

	return v, nil
}

// Compare returns -1, 0 or 1 according to the SemVer precedence of two parsed
// versions. Build metadata has no effect.
func (v Version) Compare(other Version) int {
	for _, parts := range [3][2]string{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if n := compareNumber(parts[0], parts[1]); n != 0 {
			return n
		}
	}

	switch {
	case len(v.Prerelease) == 0 && len(other.Prerelease) > 0:
		return 1
	case len(v.Prerelease) > 0 && len(other.Prerelease) == 0:
		return -1
	}

	return slices.CompareFunc(v.Prerelease, other.Prerelease, compareIdentifier)
}

func numeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func compareNumber(a, b string) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}

	return cmp.Compare(a, b)
}

func compareIdentifier(a, b string) int {
	an, bn := numeric(a), numeric(b)
	switch {
	case an && bn:
		return compareNumber(a, b)
	case an:
		return -1
	case bn:
		return 1
	default:
		return cmp.Compare(a, b)
	}
}
