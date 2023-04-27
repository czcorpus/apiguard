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
	HeaderAPIKey    = "X-Api-Key"
	HeaderAPIUserID = "X-Api-User"
)

func MapSessionCookie(req *http.Request, externalCookie, internalCookie string) error {
	ec, err := req.Cookie(externalCookie)
	if err == http.ErrNoCookie {
		return nil

	} else if err != nil {
		return fmt.Errorf("failed to map cookie %s", externalCookie)
	}

	_, err = req.Cookie(internalCookie)
	if err == nil {
		allCookies := req.Cookies()
		req.Header.Del("cookie")
		for _, c := range allCookies {
			if c.Name == internalCookie {
				c.Value = ec.Value
			}
			req.AddCookie(c)
		}

	} else {
		allCookies := req.Cookies()
		req.Header.Del("cookie")
		for _, c := range allCookies {
			if c.Name == externalCookie {
				c.Name = internalCookie
			}
			req.AddCookie(c)
		}
	}
	return nil
}
