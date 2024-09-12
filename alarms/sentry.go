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
)

// serviceEntry keeps all the information about watched service
// and its clients
type serviceEntry struct {
	Conf           AlarmConf
	limits         map[common.CheckInterval]int
	Service        string
	ClientRequests *ClientRequests
}

// serviceEntry:

func (se *serviceEntry) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	cr := se.ClientRequests.AsMap()
	err := enc.Encode(&cr)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (se *serviceEntry) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	cr := make(map[common.UserID]*UserActivity)
	err := dec.Decode(&cr)
	if err != nil {
		return err
	}
	se.ClientRequests = NewClientRequestsFrom(cr)
	return nil
}
