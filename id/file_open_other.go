// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build !unix && !windows

package id

import "os"

func openIDFile(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, errNonRegularIDFile
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	if !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, errNonRegularIDFile
	}

	return file, nil
}
