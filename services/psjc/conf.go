// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package psjc

import "fmt"

type Conf struct {
	BaseURL         string `json:"baseURL"`
	ClientUserAgent string `json:"clientUserAgent"`
	MaxQueries      int    `json:"maxQueries"`
}

func (c *Conf) Validate(context string) error {
	if c.BaseURL == "" {
		return fmt.Errorf("%s.baseURL is missing/empty", context)
	}
	return nil
}
