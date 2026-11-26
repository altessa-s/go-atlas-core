// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package id

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// File is a [Provider] that persists the Service ID to a file on disk.
//
// On the first call to [File.ID], the provider reads the first line of the
// configured regular file. The first line must be nonempty after trimming
// whitespace and at most 64 KiB before the newline. A missing file is
// atomically populated with a new 128-bit random hex ID.
//
// All methods are safe for concurrent use; initialization is guarded by a
// [sync.Once]. Two categories of errors are tracked separately:
//
//   - A fatal initialization error (e.g. unreadable file) causes [File.ID] to
//     return an empty string. Retrieve it with [File.Error].
//   - A non-fatal persistence error (e.g. unable to write a newly generated ID)
//     still allows [File.ID] to return the in-memory value. Retrieve it with
//     [File.PersistenceError].
type File struct {
	id         string    // The cached Service ID
	filePath   string    // Path to the file containing the ID
	once       sync.Once // Ensures ID is initialized only once
	initErr    error     // Stores any initialization error (fatal - prevents ID from being returned)
	persistErr error     // Stores any persistence error (non-fatal - ID still returned but not persisted)
}

// NewFile creates a new [File] provider bound to the given file path.
// The file is not accessed until the first call to [File.ID], so construction
// succeeds even when the file does not yet exist.
//
// Returns an error if filePath is the empty string.
func NewFile(filePath string) (*File, error) {
	if filePath == "" {
		return nil, fmt.Errorf("id: file path cannot be empty")
	}

	s := &File{
		filePath: filePath,
	}

	return s, nil
}

// MustNewFile is like [NewFile] but panics if the provider cannot be created.
// It is intended for program-level initialization where failure is
// unrecoverable.
func MustNewFile(filePath string) *File {
	f, err := NewFile(filePath)
	if err != nil {
		panic(err)
	}

	return f
}

// ID returns the Service ID, lazily initializing it on the first call.
//
// Initialization reads the existing file, or generates a new 128-bit random
// hex string and persists it. The result is cached so subsequent calls return
// the same value without I/O. Concurrent callers block until the first
// initialization completes.
//
// Returns an empty string if a fatal initialization error occurred; call
// [File.Error] to inspect the cause.
func (s *File) ID() string {
	s.once.Do(func() {
		s.initializeID()
	})

	if s.initErr != nil {
		// Return empty string on error instead of panicking
		// Callers can check for empty ID or use Error() method
		return ""
	}

	return s.id
}

// Error returns the fatal initialization error, if any, that prevents
// [File.ID] from returning a valid value. It triggers lazy initialization
// if it has not already occurred.
//
// Returns nil when initialization succeeded. For non-fatal write failures,
// use [File.PersistenceError] instead.
func (s *File) Error() error {
	// Ensure initialization has been attempted
	s.once.Do(func() {
		s.initializeID()
	})

	return s.initErr
}

// PersistenceError returns the non-fatal error that occurred when writing a
// newly generated ID back to disk. When this error is non-nil, [File.ID]
// still returns a valid in-memory value for the lifetime of the current
// process, but a new ID will be generated on the next restart. It triggers
// lazy initialization if it has not already occurred.
//
// Returns nil when the ID was read from an existing file or when the write
// succeeded.
func (s *File) PersistenceError() error {
	// Ensure initialization has been attempted
	s.once.Do(func() {
		s.initializeID()
	})

	return s.persistErr
}

// initializeID performs the one-time initialization of the ID.
// This method is called exactly once by sync.Once, ensuring thread safety.
func (s *File) initializeID() {
	id, err := readLine(s.filePath)
	if err == nil {
		s.id = id
		return
	}

	if !errors.Is(err, os.ErrNotExist) {
		s.initErr = coreerrs.WrapOperation(err, "read ID file "+s.filePath)
		return
	}

	id = strings.ToUpper(NewHex(idBytes))

	err = writeLine(s.filePath, id)
	if errors.Is(err, os.ErrExist) {
		// Another initializer won the race. Use the ID it published.
		id, err = readLine(s.filePath)
		s.initErr = coreerrs.WrapOperation(err, "read ID file "+s.filePath)
	} else {
		s.persistErr = coreerrs.WrapOperation(err, "persist ID to "+s.filePath)
	}

	s.id = id
}

// writeLine publishes a complete ID without replacing an existing file.
func writeLine(path, id string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".sid-*")
	if err != nil {
		return coreerrs.WrapOperation(err, "create temporary file")
	}

	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	if _, err := file.WriteString(id); err != nil {
		return coreerrs.WrapOperation(err, "write temporary file")
	}

	if err := file.Sync(); err != nil {
		return coreerrs.WrapOperation(err, "sync temporary file")
	}

	if err := file.Close(); err != nil {
		return coreerrs.WrapOperation(err, "close temporary file")
	}

	return os.Link(file.Name(), path)
}

const (
	idBytes        = 16
	maxIDLineBytes = 64 << 10
)

var (
	errNonRegularIDFile = errors.New("ID file must be a regular file")
	errEmptyID          = errors.New("ID file has an empty first line")
	errIDLineTooLong    = errors.New("ID file first line exceeds 64 KiB")
)

// readLine reads and validates the first line of an existing regular file.
func readLine(path string) (string, error) {
	file, err := openIDFile(path)
	if err != nil {
		return "", coreerrs.WrapOperation(err, "open file")
	}

	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", coreerrs.WrapOperation(err, "stat file")
	}

	if !info.Mode().IsRegular() {
		return "", errNonRegularIDFile
	}

	reader := bufio.NewReaderSize(file, maxIDLineBytes+1)

	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", errIDLineTooLong
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return "", coreerrs.WrapOperation(err, "read first line")
	}

	id := strings.TrimSpace(string(line))
	if id == "" {
		return "", errEmptyID
	}

	return id, nil
}

// Runtime check to ensure that the File type implements the Provider interface.
var _ Provider = (*File)(nil)
