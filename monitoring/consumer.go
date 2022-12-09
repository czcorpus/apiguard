// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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
	"fmt"

	"github.com/rs/zerolog/log"
)

type ConfirmMsg struct {
	Error error
}

func (cm ConfirmMsg) String() string {
	return fmt.Sprintf("ConfirmMsg{Error: %v}", cm.Error)
}

// RunWriteConsumerSync reads from incomingData channel and stores the data
// to a configured InfluxDB measurement. For performance reasons, the actual
// database write is performed each time number of added items equals
// conf.PushChunkSize and also once the incomingData channel is closed.
func RunWriteConsumerSync[T Influxable](conf *ConnectionConf, incomingData <-chan T) {
	if conf.IsConfigured() {
		var err error
		errListener := func(err error) {
			log.Error().Err(err).Send()
		}
		client, err := NewRecordWriter[T](conf, errListener)
		if err != nil {
			log.Error().Err(err).Send()
		}
		for rec := range incomingData {
			client.AddRecord(rec)
		}
		if err != nil {
			log.Error().Err(err).Send()
		}

	} else {
		for range incomingData {
		}
	}
}
