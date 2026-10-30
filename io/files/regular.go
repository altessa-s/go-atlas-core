// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package files

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotRegular is wrapped by the error [OpenRegular] returns for a path that
// is not a regular file: a FIFO, a device, a socket or a directory.
var ErrNotRegular = errors.New("not a regular file")

// OpenRegular opens path for reading and returns it only when it is a regular
// file. The file is opened without blocking (O_NONBLOCK where the platform
// has it), so opening a FIFO without a writer returns at once, and the check
// is made on the opened descriptor, so replacing the file between a check and
// the open cannot slip a FIFO or a device through. A file whose metadata
// cannot be read is refused too. Reads of a regular file are unaffected by
// the non-blocking flag.
//
// #nosec G304 -- opening a caller-chosen path is the purpose of this function; use OpenRegularIn to confine it
func OpenRegular(path string) (*os.File, error) {
	f, err := os.OpenFile(path, openRegularFlags, 0)
	if err != nil {
		return nil, err
	}
	return checkRegular(f, path)
}

// OpenRegularIn is [OpenRegular] for name inside root: the open is confined
// to root by [os.Root], so a name (or a parent directory) replaced by a
// symlink leading outside root after any earlier check fails to open instead
// of reaching the outside file.
func OpenRegularIn(root *os.Root, name string) (*os.File, error) {
	f, err := root.OpenFile(name, openRegularFlags, 0)
	if err != nil {
		return nil, err
	}
	return checkRegular(f, name)
}

// OpenRootDir opens path as an [os.Root] only when it is a directory, without
// blocking: path is opened with a trailing separator, which makes the open
// itself fail (ENOTDIR) for anything but a directory, so a FIFO swapped in for
// the directory cannot block it (os.OpenRoot alone opens the path before
// checking it is a directory).
// An empty path is refused as os.OpenRoot refuses it (it never becomes the
// filesystem root).
func OpenRootDir(path string) (*os.Root, error) {
	if path == "" {
		return os.OpenRoot(path)
	}
	if !strings.HasSuffix(path, string(filepath.Separator)) {
		path += string(filepath.Separator)
	}
	return os.OpenRoot(path)
}

// checkRegular returns f when its descriptor is a regular file and closes it
// otherwise.
func checkRegular(f *os.File, name string) (*os.File, error) {
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	if !fi.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", name, ErrNotRegular)
	}
	return f, nil
}

// ReadRegular reads the whole file at path through [OpenRegular]: a FIFO, a
// device or a socket is refused instead of blocking or never ending.
func ReadRegular(path string) ([]byte, error) {
	f, err := OpenRegular(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
