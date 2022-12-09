// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package fsops

import (
	"fmt"
	"os"
	"time"
)

func IsFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	finfo, err := f.Stat()
	if err != nil {
		return false, err
	}
	return finfo.Mode().IsRegular(), nil
}

func DeleteFile(path string) error {
	isFile, err := IsFile(path)
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}
	if !isFile {
		return fmt.Errorf("failed to delete file %s: path is not a file", path)
	}
	return os.Remove(path)
}

func GetFileMtime(filePath string) time.Time {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}
	}
	finfo, err := f.Stat()
	if err == nil {
		return time.Unix(finfo.ModTime().Unix(), 0)

	}
	return time.Time{}
}

// IsDir tests whether a provided path represents
// a directory. If not or in case of an IO error,
// false is returned.
func IsDir(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	finfo, err := f.Stat()
	if err != nil {
		return false
	}
	return finfo.Mode().IsDir()
}
