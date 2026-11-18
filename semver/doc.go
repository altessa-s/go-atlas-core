// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package semver parses semantic versions and compares their SemVer 2.0.0 precedence.
//
// # Usage
//
//	current, err := semver.Parse("v2.0.0-rc.1+build.3")
//	if err != nil {
//		return err
//	}
//	minimum, err := semver.Parse("2.0.0")
//	if err != nil {
//		return err
//	}
//	admitted := current.Compare(minimum) >= 0
package semver
