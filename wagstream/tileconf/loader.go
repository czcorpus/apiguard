// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tileconf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotFound = errors.New("tile conf not found")
)

// JSONConf is a subset of an actual tile conf. For our purposes,
// we need just proper tile ID.
type JSONConf struct {
	Ident string `json:"ident"`
}

// ------------------ JSONFiles loader

type JSONFiles struct {
	RootDir string
	files   map[string]appDirectory // maps app:Tile => fs path
}

func (jf *JSONFiles) GetConf(id string) ([]byte, error) {
	log.Info().Str("id", id).Msg("loading WaG tile configuration")
	idElms := strings.Split(id, ":") // [app]:[tile]
	if len(idElms) != 2 {
		return []byte{}, fmt.Errorf("invalid tile id '%s', must be [app]:[tile]", id)
	}
	appConf, ok := jf.files[idElms[0]]
	if !ok {
		return []byte{}, ErrNotFound
	}
	fullPath, ok := appConf.files[idElms[1]]
	if !ok {
		return []byte{}, ErrNotFound
	}
	rawConf, err := os.ReadFile(fullPath)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to read tile JSON config file %s: %w", appConf.fullPath(), err)
	}

	var conf JSONConf
	if err := json.Unmarshal(rawConf, &conf); err != nil {
		return []byte{}, fmt.Errorf("invalid tile JSON config file %s: %w", fullPath, err)
	}
	return rawConf, nil
}

func scanAppFiles(rootDir, appID string) (confFiles, error) {
	items, err := os.ReadDir(filepath.Join(rootDir, appID))
	if err != nil {
		return map[string]string{}, fmt.Errorf("failed to scan directory for wag tile configs: %w", err)
	}
	ans := make(confFiles)
	for _, item := range items {
		fullPath := filepath.Join(rootDir, appID, item.Name())
		rawConf, err := os.ReadFile(fullPath)
		if err != nil {
			log.Error().Err(err).Str("file", fullPath).Msg("failed to read tile JSON config file; skipping")
			continue
		}

		var conf JSONConf
		if err := json.Unmarshal(rawConf, &conf); err != nil {
			log.Error().Err(err).Str("file", fullPath).Msg("invalid tile JSON config file; skipping")
			continue
		}
		ans[conf.Ident] = fullPath
	}
	return ans, nil
}

func (jf *JSONFiles) scanAll(ctx context.Context, rootDir string) error {
	jf.files = make(map[string]appDirectory)
	items, err := os.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("failed to rescan tile configs: %w", err)
	}
	for _, item := range items {
		appID := item.Name()
		appDir := appDirectory{
			id:       appID,
			rootPath: rootDir,
		}
		tileMap, err := scanAppFiles(rootDir, appID)
		if err != nil {
			log.Err(err).
				Str("path", filepath.Join(rootDir, appID)).
				Msg("failed to process tile conf directory, skipping")
			continue
		}
		appDir.files = tileMap

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("failed to instantiate JSONFiles loader: %w", err)
		}
		if err := watcher.Add(filepath.Join(rootDir, appID)); err != nil {
			return fmt.Errorf("failed to instantiate JSONFiles loader: %w", err)
		}
		jf.files[appDir.id] = appDir
		log.Info().Str("directory", filepath.Join(rootDir, appID)).Msg("watching tile conf directory for changes")
		go func() {
			for {
				select {
				case <-ctx.Done():
					log.Warn().Str("app", appDir.id).Msg("closing JSONFiles file watch due to cancellation")
					watcher.Close()
				case evt, ok := <-watcher.Events:
					if !ok {
						return
					}
					updFiles, err := scanAppFiles(rootDir, appDir.id)
					if err != nil {
						log.Error().Err(err).Str("app", appID).Msg("failed to rescan directory for an app")
					}
					log.Warn().
						Str("app", appDir.id).
						Str("name", evt.Name).
						Str("operation", evt.Op.String()).
						Msg("detected change in tile JSON config file(s)")
					appDir.files = updFiles
					jf.files[appDir.id] = appDir
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Error().Err(err).Msg("JSONFiles failed to watch for file changes")

				}
			}
		}()

	}
	return nil
}

func NewJSONFiles(ctx context.Context, rootDir string) (*JSONFiles, error) {
	ans := &JSONFiles{
		RootDir: rootDir,
	}
	if err := ans.scanAll(ctx, rootDir); err != nil {
		return nil, fmt.Errorf("failed to instantiate JSONFiles: %w", err)
	}
	return ans, nil
}

// -------------------------------

type Null struct{}

func (null *Null) GetConf(id string) ([]byte, error) {
	return []byte{}, ErrNotFound
}
