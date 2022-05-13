// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package storage

import "fmt"

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
