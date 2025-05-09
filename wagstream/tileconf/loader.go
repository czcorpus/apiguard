// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package tileconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/czcorpus/cnc-gokit/fs"
)

var (
	ErrNotFound = errors.New("tile conf not found")
)

type JSONConf struct {
	Ident string `json:"ident"`
}

type JSONFiles struct {
	RootDir    string
	loadedData map[string][]byte
	mapLock    sync.Mutex
}

func (jf *JSONFiles) mkFullIdent(db, prefix, domain, tile string) string {
	return fmt.Sprintf("%s/%s:%s:%s", db, prefix, domain, tile)
}

func (jf *JSONFiles) scanFiles(db, prefix string) error {
	jf.mapLock.Lock()
	defer jf.mapLock.Unlock()
	path := filepath.Join(jf.RootDir, db, prefix)
	domainEntries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to scan for domain dirs in %s: %w", path, err)
	}
	for _, entry := range domainEntries {
		entryPath := filepath.Join(path, entry.Name())
		isDir, err := fs.IsDir(entryPath)
		if err != nil {
			return fmt.Errorf("failed to test domain etry %s for being a directory: %w", entryPath, err)
		}
		if isDir {
			err := jf.scanFilesForDomain(entryPath, db, prefix, entry.Name())
			if err != nil {
				return fmt.Errorf("failed to scan files for domain %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func (jf *JSONFiles) scanFilesForDomain(domainPath, db, prefix, domain string) error {
	items, err := os.ReadDir(domainPath)
	if err != nil {
		return fmt.Errorf("failed to scan tile conf dir %s: %w", domainPath, err)
	}
	for _, entry := range items {
		rawConf, err := os.ReadFile(filepath.Join(domainPath, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read json config file %s: %w", domainPath, err)
		}
		var conf JSONConf
		if err := json.Unmarshal(rawConf, &conf); err != nil {
			return fmt.Errorf("failed to unmarshal json config file %s: %w", domainPath, err)
		}
		jf.loadedData[jf.mkFullIdent(db, prefix, domain, conf.Ident)] = rawConf
	}

	return nil
}

func (jf *JSONFiles) GetConf(db, id string) ([]byte, error) {
	idElms := strings.Split(id, ":") // [prefix]:[domain]:[tile]
	if len(idElms) != 3 {
		return []byte{}, fmt.Errorf("invalid tile id '%s', must be [prefix]:[domain]:[tile]", id)
	}
	if jf.loadedData == nil {
		jf.loadedData = make(map[string][]byte)
		if err := jf.scanFiles(db, idElms[0]); err != nil {
			return []byte{}, fmt.Errorf("failed to get conf: %w", err)
		}
	}

	data, ok := jf.loadedData[jf.mkFullIdent(db, idElms[0], idElms[1], idElms[2])]
	if !ok {
		// let's try rescan, maybe a new file was added and now requested
		if err := jf.scanFiles(db, idElms[0]); err != nil {
			return []byte{}, fmt.Errorf("failed to get conf: %w", err)
		}
		data, ok = jf.loadedData[jf.mkFullIdent(db, idElms[0], idElms[1], idElms[2])]
		if !ok {
			return []byte{}, ErrNotFound
		}
	}
	return []byte(data), nil
}

// -------------------------------

type Null struct{}

func (null *Null) GetConf(db, id string) ([]byte, error) {
	return []byte{}, ErrNotFound
}
