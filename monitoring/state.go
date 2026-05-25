// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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

package monitoring

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

type ctxWriter struct {
	ctx context.Context
	w   io.Writer
}

func (cw *ctxWriter) Write(p []byte) (int, error) {
	select {
	case <-cw.ctx.Done():
		return 0, cw.ctx.Err()
	default:
		return cw.w.Write(p)
	}
}

// this file contains GOB encoding/decoding routines for BreachDetector and types in involves

// brdetect:

func (brdetect *BreachDetector) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	clients := brdetect.clients.AsMap()
	clients2 := make(map[string]*serviceEntry)
	for k, v := range clients {
		v2 := *v
		clients2[k] = &v2
	}
	log.Debug().Int("numClients", len(clients2)).Int("numReports", len(brdetect.reports)).Msg("saving BreachDetector state")
	err := encoder.Encode(&clients2)
	if err != nil {
		return []byte{}, err
	}
	err = encoder.Encode(&brdetect.reports)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (brdetect *BreachDetector) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	var clients map[string]*serviceEntry
	err := decoder.Decode(&clients)
	if err != nil {
		return err
	}
	brdetect.clients = collections.NewConcurrentMapFrom(clients)
	brdetect.clients.Iterate(func(service string, data *serviceEntry) bool {
		log.Info().
			Str("service", service).
			Int("numItems", data.ClientRequests.Len()).
			Msg("Loaded BreachDetector.clients")
		return true
	})

	err = decoder.Decode(&brdetect.reports)
	for _, rep := range brdetect.reports {
		rep.location = brdetect.location
	}
	log.Info().
		Int("numItems", len(brdetect.reports)).
		Msg("loaded BreachDetector.reports")
	return err
}

func SaveState(ctx context.Context, brdetect *BreachDetector) error {
	tmpFile, err := os.CreateTemp(brdetect.limitingConf.StatusDataDir, "breach-detector-state-*.gob.tmp")
	if err != nil {
		return fmt.Errorf("failed to save BreachDetector state: %w", err)
	}
	tmpPath := tmpFile.Name()
	encoder := gob.NewEncoder(&ctxWriter{ctx: ctx, w: tmpFile})
	encErr := encoder.Encode(brdetect)
	tmpFile.Close()
	if encErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save BreachDetector state: %w", encErr)
	}
	finalPath := path.Join(brdetect.limitingConf.StatusDataDir, alarmStatusFile)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save BreachDetector state: %w", err)
	}
	log.Info().
		Str("file", finalPath).
		Msg("BreachDetector runtime data saved")
	return nil
}

func LoadState(brdetect *BreachDetector) error {
	file_path := path.Join(brdetect.limitingConf.StatusDataDir, alarmStatusFile)
	is_file, err := fs.IsFile(file_path)
	if err != nil {
		return fmt.Errorf("failed to load state from file %s: %w", file_path, err)
	}
	if is_file {
		fsize, err := fs.FileSize(file_path)
		if err != nil {
			return fmt.Errorf("failed to load state from file %s: %w", file_path, err)
		}
		if fsize == 0 {
			log.Warn().Msg("encountered zero size state file, ignoring")
			return nil
		}
		file, err := os.Open(file_path)
		if err != nil {
			return fmt.Errorf("failed to load state from file %s: %w", file_path, err)
		}
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(brdetect)
		if err != nil {
			return fmt.Errorf("failed to load state from file %s: %w", file_path, err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("failed to load state from file %s: %w", file_path, err)
		}
		log.Info().Msg("Alarm attributes loaded")
	}
	return nil
}
