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
	"encoding/gob"

	"github.com/czcorpus/apiguard-common/common"
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

	cr := make(map[string]*UserActivity)
	err := dec.Decode(&cr)
	if err != nil {
		return err
	}
	se.ClientRequests = NewClientRequestsFrom(cr)
	return nil
}
