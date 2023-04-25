// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package backend

import (
	"fmt"
	"net/http"
)

const (
	HeaderAPIKey = "X-Api-Key"
)

type CookieMapping map[string]string

func (m CookieMapping) KeyOfValue(value string) (string, bool) {
	for k, v := range m {
		if v == value {
			return k, true
		}
	}
	return "", false
}

func MapCookies(req *http.Request, mapping CookieMapping) error {
	for srcCookie, dstCookie := range mapping {
		c, err := req.Cookie(srcCookie)
		if err == http.ErrNoCookie {
			continue

		} else if err != nil {
			return fmt.Errorf("failed to map cookie %s", srcCookie)
		}
		c2 := *c
		c2.Name = dstCookie
		req.AddCookie(&c2)
		c.MaxAge = -1
	}
	return nil
}
