// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package storage

import (
	"encoding/json"
	"fmt"
)

type Conf struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func (c *Conf) Validate(context string) error {
	if c.Host == "" {
		return fmt.Errorf("%s.host is empty/missing", context)
	}
	if c.User == "" {
		return fmt.Errorf("%s.user is empty/missing", context)
	}
	if c.Database == "" {
		return fmt.Errorf("%s.database is empty/missing", context)
	}
	return nil
}

type DataCleanupResult struct {
	NumDeletedStats     int
	NumDeletedActions   int
	NumDeletedBans      int
	NumDeletedDelayLogs int
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
			NumDeletedStats     int `json:"deletedStats"`
			NumDeletedActions   int `json:"deletedActions"`
			NumDeletedBans      int `json:"numDeletedBans"`
			NumDeletedDelayLogs int `json:"numDeletedDelayLogs"`

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
