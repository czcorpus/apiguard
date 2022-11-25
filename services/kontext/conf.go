// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import "fmt"

type Conf struct {
	BaseURL           string `json:"baseURL"`
	SessionCookieName string `json:"sessionCookieName"`
}

func (c *Conf) Validate(context string) error {
	if c.BaseURL == "" {
		return fmt.Errorf("%s.baseURL is missing/empty", context)
	}
	if c.SessionCookieName == "" {
		return fmt.Errorf("%s.sessionCookieName is missing/empty", context)
	}
	return nil
}
