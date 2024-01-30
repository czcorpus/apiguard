// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"net/http"
)

type ProxiedResponse struct {
	Body       []byte
	Headers    http.Header
	StatusCode int
	Err        error
	Cached     bool
}

func (pr *ProxiedResponse) GetBody() []byte {
	return pr.Body
}

func (pr *ProxiedResponse) GetHeaders() http.Header {
	return pr.Headers
}

func (pr *ProxiedResponse) GetStatusCode() int {
	return pr.StatusCode
}

func (pr *ProxiedResponse) GetError() error {
	return pr.Err
}

func (pr *ProxiedResponse) IsCached() bool {
	return pr.Cached
}

func (pr *ProxiedResponse) MarkCached() {
	pr.Cached = true
}

// SimpleResponse represents a backend response where we don't
// care about authentication and/or information returned via
// headers
type SimpleResponse struct {
	Body       []byte
	StatusCode int
	Err        error
	Cached     bool
}

func (sr *SimpleResponse) GetBody() []byte {
	return sr.Body
}

func (sr *SimpleResponse) GetHeaders() http.Header {
	return map[string][]string{}
}

func (sr *SimpleResponse) GetStatusCode() int {
	return sr.StatusCode
}

func (sr *SimpleResponse) GetError() error {
	return sr.Err
}

func (sr *SimpleResponse) IsCached() bool {
	return sr.Cached
}

func (sr *SimpleResponse) MarkCached() {
	sr.Cached = true
}
