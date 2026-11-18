// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package appinfo

import (
	"fmt"
	"strings"

	"github.com/altessa-s/go-atlas-core/semver"
)

// parseSemVer keeps historical ldflag values usable, including partial
// versions and nonnumeric development labels.
func parseSemVer(v string) (*SemanticVersion, error) {
	if parsed, err := semver.Parse(v); err == nil {
		return &SemanticVersion{Major: parsed.Major, Minor: parsed.Minor, Patch: parsed.Patch, Prerelease: parsed.Prerelease}, nil
	}

	if v == "" {
		return nil, fmt.Errorf("empty version string")
	}

	base, _, _ := strings.Cut(strings.TrimPrefix(v, "v"), "+")
	core, pre, hasPre := strings.Cut(base, "-")
	major, rest, hasMinor := strings.Cut(core, ".")

	minor, patch, hasPatch := strings.Cut(rest, ".")
	if strings.Contains(patch, ".") {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	if !hasMinor {
		minor = "0"
	}

	if !hasPatch {
		patch = "0"
	}

	result := &SemanticVersion{Major: major, Minor: minor, Patch: patch}
	if hasPre {
		result.Prerelease = strings.Split(pre, ".")
	}

	return result, nil
}
