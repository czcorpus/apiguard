// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/common"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

func (se *serviceEntry) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(se.Conf)
	if err != nil {
		return nil, err
	}
	err = enc.Encode(se.limits)
	if err != nil {
		return nil, err
	}
	err = enc.Encode(se.Service)
	if err != nil {
		return nil, err
	}
	cr := se.ClientRequests.AsMap()
	err = enc.Encode(&cr)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (se *serviceEntry) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	err := dec.Decode(&se.Conf)
	if err != nil {
		return err
	}
	err = dec.Decode(&se.limits)
	if err != nil {
		return err
	}
	err = dec.Decode(&se.Service)
	if err != nil {
		return err
	}
	cr := make(map[common.UserID]*userLimitInfo)
	err = dec.Decode(&cr)
	if err != nil {
		return err
	}
	log.Debug().
		Str("service", se.Service).
		Int("numItems", len(cr)).
		Msg("Loaded ClientRequest for serviceEntry")
	se.ClientRequests = NewClientRequestsFrom(cr)
	return nil
}

func SaveState(aticker *AlarmTicker) error {
	file, err := os.Create(path.Join(aticker.statusDataDir, alarmStatusFile))
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(aticker)
	if err != nil {
		return err
	}
	err = file.Close()
	if err == nil {
		log.Debug().Msg("Alarm attributes saved")
	}
	return err
}

func LoadState(aticker *AlarmTicker) error {
	file_path := path.Join(aticker.statusDataDir, alarmStatusFile)
	is_file, err := fs.IsFile(file_path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	if is_file {
		file, err := os.Open(file_path)
		if err != nil {
			return err
		}
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(aticker)
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
		log.Debug().Msg("Alarm attributes loaded")
	}
	return nil
}
