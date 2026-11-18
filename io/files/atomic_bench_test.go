// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package files_test

import (
	"path/filepath"
	"testing"

	"github.com/altessa-s/go-atlas-core/io/files"
)

func BenchmarkWriteFileAtomic(b *testing.B) {
	path := filepath.Join(b.TempDir(), "state")

	data := make([]byte, 4096)
	for b.Loop() {
		if err := files.WriteFileAtomic(path, data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
}
