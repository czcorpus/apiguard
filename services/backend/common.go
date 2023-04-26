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
	c, err := req.Cookie(externalCookie)
	if err == http.ErrNoCookie {
		return nil

	} else if err != nil {
		return fmt.Errorf("failed to map cookie %s", externalCookie)
	}
	c2 := *c
	c2.Name = internalCookie
	req.AddCookie(&c2)
	c.MaxAge = -1
	return nil
}
