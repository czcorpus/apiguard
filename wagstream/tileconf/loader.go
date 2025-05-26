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

func (jf *JSONFiles) mkFullIdent(db, prefix, tile string) string {
	return fmt.Sprintf("%s/%s:%s", db, prefix, tile)
}

func (jf *JSONFiles) scanFiles(db, prefix string) error {
	jf.mapLock.Lock()
	defer jf.mapLock.Unlock()
	path := filepath.Join(jf.RootDir, db, prefix)
	tileConfigs, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to scan for domain dirs in %s: %w", path, err)
	}
	for _, entry := range tileConfigs {
		entryPath := filepath.Join(path, entry.Name())
		rawConf, err := os.ReadFile(entryPath)
		if err != nil {
			return fmt.Errorf("failed to read json config file %s: %w", entryPath, err)
		}
		var conf JSONConf
		if err := json.Unmarshal(rawConf, &conf); err != nil {
			return fmt.Errorf("failed to unmarshal json config file %s: %w", entryPath, err)
		}
		jf.loadedData[jf.mkFullIdent(db, prefix, conf.Ident)] = rawConf
	}
	return nil
}

func (jf *JSONFiles) GetConf(db, id string) ([]byte, error) {
	idElms := strings.Split(id, ":") // [prefix]:[tile]
	if len(idElms) != 2 {
		return []byte{}, fmt.Errorf("invalid tile id '%s', must be [prefix]:[tile]", id)
	}
	if jf.loadedData == nil {
		jf.loadedData = make(map[string][]byte)
		if err := jf.scanFiles(db, idElms[0]); err != nil {
			return []byte{}, fmt.Errorf("failed to get conf: %w", err)
		}
	}

	data, ok := jf.loadedData[jf.mkFullIdent(db, idElms[0], idElms[1])]
	if !ok {
		// let's try rescan, maybe a new file was added and now requested
		if err := jf.scanFiles(db, idElms[0]); err != nil {
			return []byte{}, fmt.Errorf("failed to get conf: %w", err)
		}
		data, ok = jf.loadedData[jf.mkFullIdent(db, idElms[0], idElms[1])]
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
