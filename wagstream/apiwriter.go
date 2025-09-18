// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
