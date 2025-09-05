// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package interop

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// this package contains code for Proxy - Wag-stream interoperability

const (
	TileIdHeader = "x-tile-id"
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
