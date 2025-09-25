// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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

package proxy

import (
	"io"
	"net/http"
	"strings"

	"github.com/czcorpus/apiguard-common/proxy"
)

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

func (pr *BackendProxiedStreamResponse) BackendResponse() proxy.BackendResponse {
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
