// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package id

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFile_GeneratesAndPersistsID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	p, err := NewFile(path)
	require.NoError(t, err)

	id1 := p.ID()
	require.NotEmpty(t, id1, "ID empty; err=%v", p.Error())

	// New provider instance should read the same value.
	p2, err := NewFile(path)
	require.NoError(t, err)
	id2 := p2.ID()
	require.Equal(t, id1, id2)

	// Basic format sanity: 16 bytes -> 32 hex chars, uppercased.
	require.Len(t, id1, 32)
	require.Equal(t, strings.ToUpper(id1), id1, "id not uppercased: %q", id1)
}

func TestFile_FileExistsButUnreadable_FailsFast(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	require.NoError(t, os.WriteFile(path, []byte("ABC"), 0o000))
	// Best-effort: restore perms for cleanup on some platforms.
	defer func() { _ = os.Chmod(path, 0o600) }()

	p, err := NewFile(path)
	require.NoError(t, err)

	require.Empty(t, p.ID(), "ID should be empty due to unreadable file")
	require.NotNil(t, p.Error(), "Error should be non-nil due to unreadable file")
}

func TestFile_PersistenceError_ReturnsIDButTracksError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}

	// Create a read-only directory where we cannot create files
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.Mkdir(readOnlyDir, 0o500))
	// Restore permissions for cleanup
	defer func() { _ = os.Chmod(readOnlyDir, 0o700) }()

	path := filepath.Join(readOnlyDir, "service.id")

	p, err := NewFile(path)
	require.NoError(t, err)

	// ID should still be generated and returned
	id := p.ID()
	require.NotEmpty(t, id, "ID should be non-empty even with persistence failure")

	// No fatal initialization error
	require.Nil(t, p.Error(), "persistence failure is not fatal")

	// But persistence error should be tracked
	require.NotNil(t, p.PersistenceError(), "PersistenceError should be non-nil due to read-only directory")

	// Verify ID format is still valid
	require.Len(t, id, 32)
	require.Equal(t, strings.ToUpper(id), id, "id not uppercased: %q", id)
}

func TestMustNewFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	p := MustNewFile(path)
	require.NotNil(t, p)
	require.NotEmpty(t, p.ID())
}

func TestMustNewFile_EmptyPathPanics(t *testing.T) {
	defer func() {
		require.NotNil(t, recover(), "expected panic for empty file path")
	}()
	MustNewFile("")
}

// TestFile_Error_TriggersInitialization calls Error() first, on a fresh
// File that has never had ID() called, to exercise the sync.Once body from
// Error()'s own call site rather than ID()'s.
func TestFile_Error_TriggersInitialization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	p, err := NewFile(path)
	require.NoError(t, err)

	require.Nil(t, p.Error())
	require.NotEmpty(t, p.ID(), "ID should be populated once initialization ran")
}

// TestFile_PersistenceError_TriggersInitialization calls PersistenceError()
// first, on a fresh File that has never had ID() or Error() called, to
// exercise the sync.Once body from PersistenceError()'s own call site.
func TestFile_PersistenceError_TriggersInitialization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	p, err := NewFile(path)
	require.NoError(t, err)

	require.Nil(t, p.PersistenceError())
	require.NotEmpty(t, p.ID(), "ID should be populated once initialization ran")
}

// TestFile_ReadLine_LineTooLong exercises the bufio.Scanner error branch in
// readLine: a single line exceeding bufio.MaxScanTokenSize (64KB) with no
// newline overflows the scanner's token buffer and Scan() stops with
// bufio.ErrTooLong, distinct from the os.Open failure already covered by
// TestFile_FileExistsButUnreadable_FailsFast.
func TestFile_ReadLine_LineTooLong(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	huge := strings.Repeat("a", 70*1024) // no trailing newline
	require.NoError(t, os.WriteFile(path, []byte(huge), 0o600))

	p, err := NewFile(path)
	require.NoError(t, err)

	require.Empty(t, p.ID(), "ID should be empty when the existing file cannot be scanned")
	require.Error(t, p.Error())
}

func TestFile_NoPersistenceError_WhenFileWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.id")

	p, err := NewFile(path)
	require.NoError(t, err)

	id := p.ID()
	require.NotEmpty(t, id, "ID empty; err=%v", p.Error())

	// No errors should be present
	require.Nil(t, p.Error())
	require.Nil(t, p.PersistenceError())

	// Verify file was created
	_, err = os.Stat(path)
	require.False(t, os.IsNotExist(err), "ID file was not created")
}
