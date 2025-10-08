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

package wss

import (
	"bytes"
	"net/http"
)

type customResponseWriter struct {
	body       *bytes.Buffer
	statusCode int
	header     http.Header
}

func (w *customResponseWriter) Header() http.Header {
	return w.header
}

func (w *customResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *customResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *customResponseWriter) String() string {
	return w.body.String()
}

func (w *customResponseWriter) Flush() {
}
