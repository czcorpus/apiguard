// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import "fmt"

type Conf struct {
	BaseURL         string `json:"baseURL"`
	ClientUserAgent string `json:"clientUserAgent"`
	Type            string `json:"type"`
}

func (lgc *Conf) Validate(context string) error {
	if lgc.BaseURL == "" {
		return fmt.Errorf("%s.baseURL is missing/empty", context)
	}
	if lgc.ClientUserAgent == "" {
		return fmt.Errorf("%s.clientUserAgent is missing/empty", context)
	}
	return nil
}
