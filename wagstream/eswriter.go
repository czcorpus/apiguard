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
	"net/http"
	"sync"
)

// ESAPIWriter is used to collect data returned by an API
// which itself uses EventSource data stream. In such case,
// we expect two things:
//  1. The resource knows how to fill the "event" field in
//     a compatible way (Data-Tile-%d)
//  2. There is no need to parse the response, we just take it
//     and insert into our main result stream.
//
// Compared with APIWriter which stores bytes and then provides
// them at once, here we provide a channel with incoming chunks.
type ESAPIWriter struct {
	statusCode int
	headers    http.Header
	responses  chan []byte
	closeOnce  sync.Once
}

func (aw *ESAPIWriter) Responses() <-chan []byte {
	return aw.responses
}

// Close signals that the sub-stream is finished. It must be called
// once the handler writing to the writer has returned (i.e. no
// more Write calls can happen). Repeated calls are no-op.
func (aw *ESAPIWriter) Close() {
	aw.closeOnce.Do(func() {
		close(aw.responses)
	})
}

func (aw *ESAPIWriter) Header() http.Header {
	return aw.headers
}

func (aw *ESAPIWriter) Write(data []byte) (int, error) {
	var buffer bytes.Buffer
	if aw.statusCode == 0 {
		aw.statusCode = http.StatusOK
	}
	nWritten, err := buffer.Write(data)
	if err != nil {
		return 0, err
	}
	aw.responses <- buffer.Bytes()
	return nWritten, nil
}

func (aw *ESAPIWriter) WriteHeader(statusCode int) {
	aw.statusCode = statusCode
}

func (aw *ESAPIWriter) StatusCode() int {
	return aw.statusCode
}

func (aw *ESAPIWriter) IsNotErrorStatus() bool {
	return aw.statusCode >= 200 && aw.statusCode < 300
}

// Flush is a no-op as each Write is passed to the channel
// immediately. Handlers may call it any number of times
// (e.g. after each SSE event). To signal the end of the stream,
// Close must be used.
func (aw *ESAPIWriter) Flush() {
}

func NewESAPIWriter(chanBuffSize int) *ESAPIWriter {
	return &ESAPIWriter{
		headers:   make(http.Header),
		responses: make(chan []byte, chanBuffSize),
	}
}
