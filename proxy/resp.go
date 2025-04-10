// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"io"
	"net/http"
)

type EmptyReadCloser struct{}

func (rc EmptyReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (rc EmptyReadCloser) Close() error {
	return nil
}

// ----------------------

type ProxiedResponse struct {
	BodyReader io.ReadCloser
	Headers    http.Header
	StatusCode int
	Err        error
	Cached     bool
}

func (pr *ProxiedResponse) GetBodyReader() io.ReadCloser {
	return pr.BodyReader
}

func (pr *ProxiedResponse) CloseBodyReader() error {
	return pr.BodyReader.Close()
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

// -----------------------------------------

type ProxiedStreamResponse struct {
	BodyReader io.ReadCloser
	Headers    http.Header
	StatusCode int
	Err        error
	Cached     bool
}

func (pr *ProxiedStreamResponse) GetBodyReader() io.ReadCloser {
	return pr.BodyReader
}

func (pr *ProxiedStreamResponse) CloseBodyReader() error {
	return pr.BodyReader.Close()
}

func (pr *ProxiedStreamResponse) GetBody() []byte {
	bd, err := io.ReadAll(pr.BodyReader)
	if err != nil {
		pr.Err = err
	}
	return bd
}

func (pr *ProxiedStreamResponse) GetHeaders() http.Header {
	return pr.Headers
}

func (pr *ProxiedStreamResponse) GetStatusCode() int {
	return pr.StatusCode
}

func (pr *ProxiedStreamResponse) GetError() error {
	return pr.Err
}

func (pr *ProxiedStreamResponse) IsCached() bool {
	return pr.Cached
}

func (pr *ProxiedStreamResponse) MarkCached() {
	pr.Cached = true
}

// -----------------------------------------

// SimpleResponse represents a backend response where we don't
// care about authentication and/or information returned via
// headers
type SimpleResponse struct {
	BodyReader io.ReadCloser
	StatusCode int
	Err        error
	Cached     bool
}

func (sr *SimpleResponse) GetBodyReader() io.ReadCloser {
	return sr.BodyReader
}

func (sr *SimpleResponse) CloseBodyReader() error {
	return sr.BodyReader.Close()
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
