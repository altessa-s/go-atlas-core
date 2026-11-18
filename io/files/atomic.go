// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package files

import (
	"io/fs"
	"os"
	"path/filepath"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// WriteFileAtomic writes data to a temporary file beside path, applies perm,
// syncs and closes the file, then replaces path using os.Rename. The parent
// directory must exist and is not synced. Atomicity follows os.Rename's
// platform guarantees.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".atomic-*")
	if err != nil {
		return coreerrs.WrapOperationWithContext(err, "write file atomically", path)
	}

	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(perm)
	}

	if err == nil {
		err = tmp.Sync()
	}

	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}

	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}

	if err != nil {
		_ = os.Remove(tmp.Name())
	}

	return coreerrs.WrapOperationWithContext(err, "write file atomically", path)
}
