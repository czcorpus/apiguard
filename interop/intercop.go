// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
//
//	Faculty of Arts, Charles University
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

package interop

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// this package contains code for Proxy - Wag-stream interoperability

const (
	TileIdHeader   = "x-tile-id"
	QueryIdxHeader = "x-query-idx"
)

var (
	ErrNoTileId = errors.New("no tile id in HTTP header")
)

func TileIdFromReq(req *http.Request) (int, error) {
	v := req.Header.Get(TileIdHeader)
	if v == "" {
		return -1, ErrNoTileId
	}
	ans, err := strconv.Atoi(v)
	if err != nil {
		return -1, fmt.Errorf("invalid tile id in HTTP header: %w", err)
	}
	return ans, nil
}

func QueryIdxFromReq(req *http.Request) (int, error) {
	v := req.Header.Get(QueryIdxHeader)
	if v == "" {
		return 0, nil
	}
	ans, err := strconv.Atoi(v)
	if err != nil {
		return -1, fmt.Errorf("invalid query idx in HTTP header: %w", err)
	}
	return ans, nil
}
