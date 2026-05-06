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

// this file contains GOB encoding/decoding routines for AlarmTicker and types in involves

// aticker:

func (aticker *AlarmTicker) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	clients := aticker.clients.AsMap()
	clients2 := make(map[string]*serviceEntry)
	for k, v := range clients {
		v2 := *v
		clients2[k] = &v2
	}
	log.Debug().Int("numClients", len(clients2)).Int("numReports", len(aticker.reports)).Msg("saving AlarmTicker state")
	err := encoder.Encode(&clients2)
	if err != nil {
		return []byte{}, err
	}
	err = encoder.Encode(&aticker.reports)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (aticker *AlarmTicker) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	var clients map[string]*serviceEntry
	err := decoder.Decode(&clients)
	if err != nil {
		return err
	}
	aticker.clients = collections.NewConcurrentMapFrom(clients)
	aticker.clients.ForEach(func(service string, data *serviceEntry, ok bool) {
		if !ok {
			return
		}
		log.Info().
			Str("service", service).
			Int("numItems", data.ClientRequests.Len()).
			Msg("Loaded AlarmTicker.clients")
	})

	err = decoder.Decode(&aticker.reports)
	for _, rep := range aticker.reports {
		rep.location = aticker.location
	}
	log.Info().
		Int("numItems", len(aticker.reports)).
		Msg("loaded AlarmTicker.reports")
	return err
}

func SaveState(ctx context.Context, aticker *AlarmTicker) error {
	tmpFile, err := os.CreateTemp(aticker.limitingConf.StatusDataDir, "alarm-status-*.gob.tmp")
	if err != nil {
		return fmt.Errorf("failed to save AlarmTicker state: %w", err)
	}
	tmpPath := tmpFile.Name()
	encoder := gob.NewEncoder(&ctxWriter{ctx: ctx, w: tmpFile})
	encErr := encoder.Encode(aticker)
	tmpFile.Close()
	if encErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save AlarmTicker state: %w", encErr)
	}
	finalPath := path.Join(aticker.limitingConf.StatusDataDir, alarmStatusFile)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save AlarmTicker state: %w", err)
	}
	log.Info().
		Str("file", finalPath).
		Msg("AlarmTicker runtime data saved")
	return nil
}

func LoadState(aticker *AlarmTicker) error {
	file_path := path.Join(aticker.limitingConf.StatusDataDir, alarmStatusFile)
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
		err = decoder.Decode(aticker)
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
