// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package appinfo

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

// TestSemVersionNilSver exercises the defensive nil-sver branch of
// SemVersion. sver is populated at init time from the default build
// Version and is never nil in normal operation; a failing Version string
// cannot be injected from outside the package, so this test mutates the
// unexported package state directly. Not parallel: mutates package state.
func TestSemVersionNilSver(t *testing.T) {
	saved := sver
	sver = nil
	t.Cleanup(func() { sver = saved })

	require.Equal(t, &SemanticVersion{}, SemVersion())
}

// TestAppVersionTemplateError exercises the defensive error-handling branch
// of AppVersion when template execution fails. The package's built-in
// template never fails against the map it is given, so this test swaps in
// a template that errors on execution (a field access on a string value).
// Not parallel: mutates package state.
func TestAppVersionTemplateError(t *testing.T) {
	saved := versionInfoTemplate
	versionInfoTemplate = template.Must(template.New("version").Parse(`{{.program.NoSuchField}}`))
	t.Cleanup(func() { versionInfoTemplate = saved })

	require.Equal(t, "", AppVersion())
}
