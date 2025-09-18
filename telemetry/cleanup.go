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

package telemetry

import (
	"encoding/json"
)

type DataCleanupResult struct {
	NumDeletedStats     int64
	NumDeletedActions   int64
	NumDeletedBans      int64
	NumDeletedDelayLogs int64
	Error               error
}

func (dcr DataCleanupResult) MarshalJSON() ([]byte, error) {
	var statusErr *string
	if dcr.Error != nil {
		tmp := dcr.Error.Error()
		statusErr = &tmp
	}
	return json.Marshal(
		struct {
			NumDeletedStats     int64 `json:"deletedStats"`
			NumDeletedActions   int64 `json:"deletedActions"`
			NumDeletedBans      int64 `json:"numDeletedBans"`
			NumDeletedDelayLogs int64 `json:"numDeletedDelayLogs"`

			Error *string `json:"error"`
		}{
			NumDeletedStats:     dcr.NumDeletedStats,
			NumDeletedActions:   dcr.NumDeletedActions,
			NumDeletedBans:      dcr.NumDeletedBans,
			NumDeletedDelayLogs: dcr.NumDeletedDelayLogs,
			Error:               statusErr,
		},
	)
}
