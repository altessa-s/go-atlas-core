// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package id

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func openIDFile(path string) (*os.File, error) {
	name, err := idFilePathUTF16(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	// Share deletion so the publisher can remove its temporary hard link
	// while another initializer is reading the published name.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	file := os.NewFile(uintptr(handle), path)

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = file.Close()
		return nil, err
	}

	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = file.Close()
		return nil, errNonRegularIDFile
	}

	return file, nil
}

// idFilePathUTF16 supports paths longer than the legacy Win32 limit.
func idFilePathUTF16(path string) (*uint16, error) {
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\??\`) || strings.HasPrefix(path, `\\.\`) {
		return windows.UTF16PtrFromString(path)
	}

	fullPath, err := windows.FullPath(path)
	if err != nil {
		return nil, err
	}

	const maxLegacyPath = 260
	if len(fullPath) >= maxLegacyPath {
		if unc, ok := strings.CutPrefix(fullPath, `\\`); ok {
			return windows.UTF16PtrFromString(`\\?\UNC\` + unc)
		}

		return windows.UTF16PtrFromString(`\\?\` + fullPath)
	}

	return windows.UTF16PtrFromString(fullPath)
}
