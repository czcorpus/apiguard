// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

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
		return []byte{}, fmt.Errorf("cannot load tile conf JSON %s: app %s has no directory", id, idElms[0])
	}
	fullPath, ok := appConf.files[idElms[1]]
	if !ok {
		return []byte{}, fmt.Errorf("cannot load tile conf JSON %s: tile %s not found", id, idElms[1])
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

type appDirectory struct {
	rootPath string
	id       string
	files    map[string]string
}

func (dir appDirectory) fullPath() string {
	return filepath.Join(dir.rootPath, dir.id)
}

func scanAppFiles(rootDir, appID string) (map[string]string, error) {
	items, err := os.ReadDir(filepath.Join(rootDir, appID))
	if err != nil {
		return map[string]string{}, fmt.Errorf("failed to scan directory for wag tile configs: %w", err)
	}
	ans := make(map[string]string)
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

func scanAll(ctx context.Context, rootDir string) ([]appDirectory, error) {
	ans := make([]appDirectory, 0, 10)
	items, err := os.ReadDir(rootDir)
	if err != nil {
		return []appDirectory{}, fmt.Errorf("failed to rescan tile configs: %w", err)
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
			return nil, fmt.Errorf("failed to instantiate JSONFiles loader: %w", err)
		}
		if err := watcher.Add(filepath.Join(rootDir, appID)); err != nil {
			return nil, fmt.Errorf("failed to instantiate JSONFiles loader: %w", err)
		}
		ans = append(ans, appDir)
		log.Info().Str("directory", filepath.Join(rootDir, appID)).Msg("watching tile conf directory for changes")
		go func() {
			for {
				select {
				case <-ctx.Done():
					log.Warn().Str("app", appDir.id).Msg("closing JSONFiles file watch due to cancellation")
					watcher.Close()
				case _, ok := <-watcher.Events:
					if !ok {
						return
					}
					updFiles, err := scanAppFiles(rootDir, appDir.id)
					if err != nil {
						log.Error().Err(err).Str("app", appID).Msg("failed to rescan directory for an app")
					}
					appDir.files = updFiles
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Error().Err(err).Msg("JSONFiles failed to watch for file changes")

				}
			}
		}()

	}
	return ans, nil
}

func NewJSONFiles(ctx context.Context, rootDir string) (*JSONFiles, error) {
	appDirs, err := scanAll(ctx, rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate JSONFiles: %w", err)
	}
	appDirsMap := make(map[string]appDirectory)
	for _, v := range appDirs {
		appDirsMap[v.id] = v
	}
	return &JSONFiles{
		RootDir: rootDir,
		files:   appDirsMap,
	}, nil
}

// -------------------------------

type Null struct{}

func (null *Null) GetConf(id string) ([]byte, error) {
	return []byte{}, ErrNotFound
}
