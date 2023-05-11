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
	"apiguard/monitoring/influx"
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
// to via a provided InfluxDBAdapter ('db' arg.). In case 'db' is nil, the
// function just listens to 'incomingData' and does nothing.
func RunWriteConsumerSync[T influx.Influxable](
	db *influx.InfluxDBAdapter,
	measurement string,
	incomingData <-chan T,
) {
	if db != nil {
		var err error
		client := NewRecordWriter[T](db)
		for rec := range incomingData {
			client.AddRecord(rec, measurement)
		}
		if err != nil {
			log.Error().Err(err).Msg("Failed to write influxDB record")
		}

	} else {
		for range incomingData {
		}
	}
}
