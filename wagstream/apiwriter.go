// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"bytes"
	"encoding/base64"
	"net/http"
)

// APIWriter is used to collect data returned by an API
// when we manually handle the API request using GIN's
// router we normally use in the "proxy" mode.
type APIWriter struct {
	buffer     *bytes.Buffer
	statusCode int
	headers    http.Header
}

func (aw *APIWriter) Header() http.Header {
	return aw.headers
}

func (aw *APIWriter) Write(data []byte) (int, error) {
	if aw.statusCode == 0 {
		aw.statusCode = http.StatusOK
	}
	return aw.buffer.Write(data)
}

func (aw *APIWriter) WriteHeader(statusCode int) {
	aw.statusCode = statusCode
}

func (aw *APIWriter) GetRawBytes() []byte {
	return aw.buffer.Bytes()
}

func (aw *APIWriter) GetAsBase64() []byte {
	var writer bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &writer)
	enc.Write(aw.buffer.Bytes())
	return writer.Bytes()
}

func (aw *APIWriter) StatusCode() int {
	return aw.statusCode
}

func (aw *APIWriter) Flush() {

}

func (aw *APIWriter) IsNotErrorStatus() bool {
	return aw.statusCode >= 200 && aw.statusCode < 300
}

func NewAPIWriter() *APIWriter {
	return &APIWriter{
		buffer:  new(bytes.Buffer),
		headers: make(http.Header),
	}
}
