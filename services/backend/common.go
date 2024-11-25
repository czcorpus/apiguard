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

// MapFrontendCookieToBackend takes current user's frontend authentication
// (based on the frontendCookie) and passes it as backend authentication
// (to an API server) under the backend cookie name.
// The function expects that the frontend cookie is already set as otherwise
// there would be noting to map from. This is typically fulfilled either by user
// visiting already other CNC applications or by sequence "preflight" -> "login"
// performed by a compatible application APIGuard is attached to (mostly WaG).
func MapFrontendCookieToBackend(req *http.Request, frontendCookie, backendCookie string) error {
	ec, err := req.Cookie(frontendCookie)
	if err == http.ErrNoCookie {
		return nil

	} else if err != nil {
		return fmt.Errorf("failed to map cookie %s", frontendCookie)
	}

	_, err = req.Cookie(backendCookie)
	if err == nil {
		allCookies := req.Cookies()
		req.Header.Del("cookie")
		for _, c := range allCookies {
			if c.Name == backendCookie {
				c.Value = ec.Value
			}
			req.AddCookie(c)
		}

	} else {
		allCookies := req.Cookies()
		req.Header.Del("cookie")
		for _, c := range allCookies {
			if c.Name == frontendCookie {
				c.Name = backendCookie
			}
			req.AddCookie(c)
		}
	}
	return nil
}
