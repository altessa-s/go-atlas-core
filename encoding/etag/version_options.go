// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package etag

//go:generate optgen generate --type=versionOptions --option-type=VersionOption --output=version_options_gen.go

// versionOptions tunes [ParseVersion] and [CheckVersion]. Configure it
// through the generated [VersionOption] values.
type versionOptions struct {
	// singleton accepts W/"0", the etag of a singleton resource that has not
	// been saved yet; every other resource has a positive stored version.
	singleton bool
}
