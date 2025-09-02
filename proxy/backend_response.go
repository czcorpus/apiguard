// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type EmptyReadCloser struct{}

func (rc EmptyReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (rc EmptyReadCloser) Close() error {
	return nil
}

// ----------------------

type BackendZeroResponse struct {
}

func (sr *BackendZeroResponse) GetBodyReader() io.ReadCloser {
	return &EmptyReadCloser{}
}

func (sr *BackendZeroResponse) CloseBodyReader() error {
	return nil
}

func (sr *BackendZeroResponse) GetHeaders() http.Header {
	return map[string][]string{}
}

func (sr *BackendZeroResponse) GetStatusCode() int {
	return 0
}

func (sr *BackendZeroResponse) Error() error {
	return fmt.Errorf("the response is undefined")
}

func (sr *BackendZeroResponse) IsDataStream() bool {
	return false
}

// ----------------------

type BackendProxiedResponse struct {
	BodyReader io.ReadCloser
	Headers    http.Header
	StatusCode int
	Err        error
}

func (pr *BackendProxiedResponse) GetBodyReader() io.ReadCloser {
	return pr.BodyReader
}

func (pr *BackendProxiedResponse) CloseBodyReader() error {
	return pr.BodyReader.Close()
}

func (pr *BackendProxiedResponse) GetHeaders() http.Header {
	return pr.Headers
}

func (pr *BackendProxiedResponse) GetStatusCode() int {
	return pr.StatusCode
}

func (pr *BackendProxiedResponse) Error() error {
	return pr.Err
}

func (pr *BackendProxiedResponse) IsDataStream() bool {
	return strings.Contains(pr.Headers.Get("Content-Type"), "text/event-stream")
}

// -----------------------------------------

type BackendProxiedStreamResponse struct {
	BodyReader io.ReadCloser
	Headers    http.Header
	StatusCode int
	Err        error

	// readData keeps read data in case GetBody was called. This ensures
	// that it is still possible to call WriteResponse method.
	readData []byte
}

func (pr *BackendProxiedStreamResponse) BackendResponse() BackendResponse {
	return pr
}

func (pr *BackendProxiedStreamResponse) GetBodyReader() io.ReadCloser {
	return pr.BodyReader
}

func (pr *BackendProxiedStreamResponse) CloseBodyReader() error {
	return pr.BodyReader.Close()
}

func (pr *BackendProxiedStreamResponse) GetHeaders() http.Header {
	return pr.Headers
}

func (pr *BackendProxiedStreamResponse) GetStatusCode() int {
	return pr.StatusCode
}

func (pr *BackendProxiedStreamResponse) Error() error {
	return pr.Err
}

func (pr *BackendProxiedStreamResponse) IsDataStream() bool {
	return strings.Contains(pr.Headers.Get("Content-Type"), "text/event-stream")
}

// -----------------------------------------

// BackendSimpleResponse represents a backend response where we don't
// care about authentication and/or information returned via
// headers
type BackendSimpleResponse struct {
	BodyReader io.ReadCloser
	StatusCode int
	Err        error
}

func (sr *BackendSimpleResponse) GetBodyReader() io.ReadCloser {
	return sr.BodyReader
}

func (sr *BackendSimpleResponse) CloseBodyReader() error {
	return sr.BodyReader.Close()
}

func (sr *BackendSimpleResponse) GetHeaders() http.Header {
	return map[string][]string{}
}

func (sr *BackendSimpleResponse) GetStatusCode() int {
	return sr.StatusCode
}

func (sr *BackendSimpleResponse) Error() error {
	return sr.Err
}

func (sr *BackendSimpleResponse) IsDataStream() bool {
	return false
}
