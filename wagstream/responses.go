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

import "time"

// StreamingReadyResp represents a single data source response
// as passed by APIGuard's data stream.
type StreamingReadyResp struct {
	TileID int

	// QueryIdx specifies query number. In the "single" mode WaG search, this
	// must be always 0. In the "cmp" mode, it corresponds queries user entered
	// for comparison.
	QueryIdx int

	// Source is a unique identifier specifying requested data. Naturally,
	// original APIGuard URL which would be used in the "proxy" mode,
	// is the best solution for this. Such value is easy to register
	// by WaG API clients which would use such URL anyway.
	Source string

	// Data returned by an API. The format depends on the API and possibly
	// on the fact whether the client required base64 encoding for returned
	// data.
	Data []byte

	// Status contains the original HTTP status code as obtained
	// from an API
	Status int
}

// RawStreamingReadyResp is a response from a service which already produces
// EventSource data (e.g. chunked time distrib in MQuery).
type RawStreamingReadyResp struct {
	TileID int

	QueryIdx int

	Data []byte
	// Status contains the original HTTP status code as obtained
	// from an API
	Status int
}

// PingResp is used to keep the stream active in situations where - due
// to too long waiting time for a next event - there is a threat that
// the stream will be closed.
type PingResp struct {
	TS time.Time
}

type streamingError struct {
	Error string `json:"error"`
}
