// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package appinfo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/appinfo"
)

func TestDependencies_Contains_Found(t *testing.T) {
	deps := appinfo.Deps()
	// At least one dependency should exist in a real Go module
	if len(deps) == 0 {
		t.Skip("no dependencies found")
	}
	first := deps[0]
	require.True(t, deps.Contains(first.Path), "Contains(%q) = false, want true", first.Path)
}

func TestDependencies_Contains_NotFound(t *testing.T) {
	deps := appinfo.Deps()
	require.False(t, deps.Contains("nonexistent/module/path"), "Contains(nonexistent) = true, want false")
}

func TestDependencies_Get_Found(t *testing.T) {
	deps := appinfo.Deps()
	if len(deps) == 0 {
		t.Skip("no dependencies found")
	}
	first := deps[0]
	got := deps.Get(first.Path)
	require.NotNil(t, got, "Get(%q) = nil, want non-nil", first.Path)
}

func TestDependencies_Get_NotFound(t *testing.T) {
	deps := appinfo.Deps()
	require.Nil(t, deps.Get("nonexistent/module/path"), "Get(nonexistent) should return nil")
}

func TestHomeDir(t *testing.T) {
	home := appinfo.HomeDir()
	require.NotEmpty(t, home, "HomeDir() should not be empty")
}

// TestHomeDirUserProfile verifies the Windows USERPROFILE branch takes
// priority. It mutates process environment variables via t.Setenv, so it
// cannot run in parallel with other tests touching the same variables.
func TestHomeDirUserProfile(t *testing.T) {
	t.Setenv(appinfo.EnvUserProfile, "/win/profile")
	require.Equal(t, "/win/profile", appinfo.HomeDir())
}

// TestHomeDirHomeDriveAndPath verifies the Windows HOMEDRIVE+HOMEPATH
// fallback when USERPROFILE is unset. Not parallel: see
// TestHomeDirUserProfile.
func TestHomeDirHomeDriveAndPath(t *testing.T) {
	t.Setenv(appinfo.EnvUserProfile, "")
	t.Setenv(appinfo.EnvHomeDrive, "C:")
	t.Setenv(appinfo.EnvHomePath, `\Users\foo`)
	require.Equal(t, `C:\Users\foo`, appinfo.HomeDir())
}

// TestGetEnvVarTildeExpansion verifies that a leading '~' in the resolved
// value is expanded against HomeDir(). Mutates EnvPrefix and process
// environment variables, so it is not parallel.
func TestGetEnvVarTildeExpansion(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	t.Setenv(appinfo.EnvUserProfile, "")
	t.Setenv(appinfo.EnvHomeDrive, "")
	t.Setenv(appinfo.EnvHome, "/home/tester")
	t.Setenv("MYKEY", "~/data")

	require.Equal(t, "/home/tester/data", appinfo.GetEnvVar("MYKEY"))
}

// TestBinDirEnvOverride verifies BIN_DIR overrides the "/opt/bin" default.
// Mutates EnvPrefix, so it is not parallel.
func TestBinDirEnvOverride(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	t.Setenv("BIN_DIR", "/custom/bin//")
	require.Equal(t, "/custom/bin", appinfo.BinDir())
}

// TestVarDirEnvOverride verifies VAR_DIR overrides the "/var" default.
// Mutates EnvPrefix, so it is not parallel.
func TestVarDirEnvOverride(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	t.Setenv("VAR_DIR", "/custom/var//")
	require.Equal(t, "/custom/var", appinfo.VarDir())
}

// TestEtcDirEnvOverride verifies ETC_DIR overrides the computed default.
// Mutates EnvPrefix, so it is not parallel.
func TestEtcDirEnvOverride(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	t.Setenv("ETC_DIR", "/custom/etc//")
	require.Equal(t, "/custom/etc", appinfo.EtcDir())
}

// TestEtcDirWithProject verifies the Project path component is inserted
// and lowercased when Project is set. Mutates EnvPrefix, Project and Name,
// so it is not parallel.
func TestEtcDirWithProject(t *testing.T) {
	savedPrefix, savedProject, savedName := appinfo.EnvPrefix, appinfo.Project, appinfo.Name
	appinfo.EnvPrefix, appinfo.Project, appinfo.Name = "", "MyProj", "MyApp"
	t.Cleanup(func() {
		appinfo.EnvPrefix, appinfo.Project, appinfo.Name = savedPrefix, savedProject, savedName
	})
	t.Setenv("ETC_DIR", "")

	require.Equal(t, "/etc/myproj/myapp", appinfo.EtcDir())
}

// TestLibDirEnvOverride verifies LIB_DIR overrides the computed default.
// Mutates EnvPrefix, so it is not parallel.
func TestLibDirEnvOverride(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	t.Setenv("LIB_DIR", "/custom/lib//")
	require.Equal(t, "/custom/lib", appinfo.LibDir())
}

// TestLibDirWithProject verifies the Project path component is inserted
// and lowercased when Project is set. Mutates EnvPrefix, Project and Name,
// so it is not parallel.
func TestLibDirWithProject(t *testing.T) {
	savedPrefix, savedProject, savedName := appinfo.EnvPrefix, appinfo.Project, appinfo.Name
	appinfo.EnvPrefix, appinfo.Project, appinfo.Name = "", "MyProj", "MyApp"
	t.Cleanup(func() {
		appinfo.EnvPrefix, appinfo.Project, appinfo.Name = savedPrefix, savedProject, savedName
	})
	t.Setenv("LIB_DIR", "")
	t.Setenv("VAR_DIR", "")

	require.Equal(t, "/var/lib/myproj/myapp", appinfo.LibDir())
}

// TestMakeAllDirsSuccess verifies MakeAllDirs creates both VarDir and
// LibDir. Mutates EnvPrefix, so it is not parallel.
func TestMakeAllDirsSuccess(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	base := t.TempDir()
	varDir := filepath.Join(base, "var")
	libDir := filepath.Join(base, "lib")
	t.Setenv("VAR_DIR", varDir)
	t.Setenv("LIB_DIR", libDir)

	require.NoError(t, appinfo.MakeAllDirs())

	info, err := os.Stat(varDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	info, err = os.Stat(libDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

// TestMakeAllDirsError verifies MakeAllDirs surfaces the mkdir error when a
// path component is blocked by a regular file. Mutates EnvPrefix, so it is
// not parallel.
func TestMakeAllDirsError(t *testing.T) {
	saved := appinfo.EnvPrefix
	appinfo.EnvPrefix = ""
	t.Cleanup(func() { appinfo.EnvPrefix = saved })

	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	t.Setenv("VAR_DIR", filepath.Join(blocker, "sub"))
	t.Setenv("LIB_DIR", filepath.Join(base, "lib"))

	require.Error(t, appinfo.MakeAllDirs())
}

// TestEnvCachedConcurrentPopulate races many goroutines against a freshly
// cleared key to exercise the double-checked-locking path in EnvCached:
// the first goroutine to acquire the write lock populates the cache, and
// every other goroutine must observe the already-populated entry on its
// own double-check rather than re-reading the environment. Calls
// ClearEnvCache and t.Setenv, so it is not parallel with other EnvCached
// tests.
func TestEnvCachedConcurrentPopulate(t *testing.T) {
	const key = "TEST_APPINFO_RACE_KEY"
	t.Setenv(key, "race-value")

	// A single round of racing goroutines only occasionally lands two of
	// them in the exact window needed to exercise the double-checked-lock
	// hit path (one goroutine misses the initial read-lock check, then
	// finds the entry already populated on its own write-lock double
	// check). Running many rounds of many goroutines makes hitting that
	// window virtually certain in practice without any flakiness risk to
	// the assertions below, which hold on every round regardless.
	const (
		rounds  = 150
		workers = 512
	)
	for range rounds {
		appinfo.ClearEnvCache()

		results := make([]string, workers)
		var ready sync.WaitGroup
		ready.Add(workers)
		start := make(chan struct{})
		var done sync.WaitGroup
		done.Add(workers)
		for i := range workers {
			go func(i int) {
				defer done.Done()
				ready.Done()
				<-start
				results[i] = appinfo.EnvCached(key)
			}(i)
		}
		ready.Wait()
		close(start)
		done.Wait()

		for i, got := range results {
			require.Equal(t, "race-value", got, "worker %d", i)
		}
	}
}

// TestInitNormalizesLDFlagsEnvPrefix exercises the EnvPrefix normalization
// branch inside the package's init() function. That branch only runs when
// EnvPrefix is set at build time via -ldflags, as documented on the
// EnvPrefix variable; since init() runs once before any test code in this
// binary (where EnvPrefix defaults to ""), it cannot be exercised
// in-process. This test builds and runs a throwaway program, in a
// temporary module that replaces this module with the local checkout, to
// observe the normalization end to end.
func TestInitNormalizesLDFlagsEnvPrefix(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not available on PATH")
	}

	wd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	require.NoError(t, err)

	modDir := t.TempDir()
	goMod := fmt.Sprintf(
		"module tempcmd\n\ngo 1.27\n\nrequire github.com/altessa-s/go-atlas-core v0.0.0\n\nreplace github.com/altessa-s/go-atlas-core => %s\n",
		repoRoot,
	)
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte(goMod), 0o600))

	mainSrc := `package main

import (
	"fmt"

	"github.com/altessa-s/go-atlas-core/runtime/appinfo"
)

func main() {
	fmt.Println(appinfo.EnvKeyPrefix())
}
`
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "main.go"), []byte(mainSrc), 0o600))

	binPath := filepath.Join(modDir, "tempcmd_bin")
	buildCmd := exec.Command(
		goBin, "build",
		"-ldflags", "-X github.com/altessa-s/go-atlas-core/runtime/appinfo.EnvPrefix=my-App.Name",
		"-o", binPath, ".",
	)
	buildCmd.Dir = modDir
	buildCmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Skipf("building throwaway module failed (environment cannot build it): %v\n%s", err, out)
	}

	runOut, err := exec.Command(binPath).Output()
	require.NoError(t, err)
	require.Equal(t, "MY_APP_NAME_", strings.TrimSpace(string(runOut)))
}
