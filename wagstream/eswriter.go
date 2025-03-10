// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"bytes"
	"net/http"
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
}

func (aw *ESAPIWriter) Responses() <-chan []byte {
	return aw.responses
}

func (aw *ESAPIWriter) Close() {
	close(aw.responses)
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

// Flush here must be called explicitly by us to make
// sure the channel is closed. Otherwise, the total
// EventSource response would get broken.
func (aw *ESAPIWriter) Flush() {
	close(aw.responses)
}

func NewESAPIWriter(chanBuffSize int) *ESAPIWriter {
	return &ESAPIWriter{
		headers:   make(http.Header),
		responses: make(chan []byte, chanBuffSize),
	}
}
