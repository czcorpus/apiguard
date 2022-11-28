// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package rdelay

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
