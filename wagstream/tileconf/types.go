// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package tileconf

import (
	"path/filepath"
)

type confFiles map[string]string

// -----

type appDirectory struct {
	rootPath string
	id       string
	files    confFiles
}

func (dir appDirectory) fullPath() string {
	return filepath.Join(dir.rootPath, dir.id)
}
