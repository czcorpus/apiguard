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
	// HeaderAPIKey allows KonText to access API session ID
	HeaderAPIKey = "X-Api-Key"

	// HeaderAPIUserID allows for passing userId to a target API
	// backend so it knows who is actually using the API even if
	// the API itself does not have required authentication ability.
	HeaderAPIUserID = "X-Api-User"

	// HeaderIndirectCall indicates that the API query is not called
	// directly by a user but rather by an application which is queried
	// in its own way and which - based on its user query - produces
	// an API call proxied by APIGuard.
	// We are using this flag to distinguish direct API usage from
	// indirect one which has its significance e.g. when reporting
	// services usage.
	HeaderIndirectCall = "X-Indirect-Call"
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
