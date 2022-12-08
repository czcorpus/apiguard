// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"net/http"
	"time"
)

type BackendResponse interface {
	GetBody() []byte
	GetHeaders() http.Header
	GetStatusCode() int
	IsCached() bool
	GetError() error
}

type Cache interface {
	Get(req *http.Request) (BackendResponse, error)
	Set(req *http.Request, resp BackendResponse) error
}

type GlobalContext struct {
	TimezoneLocation *time.Location
}
