// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"fmt"
)

// request is a single API request which we pack into
// an HTTP stream request.
// In WaG, the corresponding types are:
// - TileRequest
// - OtherTileRequest
type request struct {
	TileID int `json:"tileId"`

	// QueryIdx specifies query number. In the "single" mode WaG search, this
	// must be always 0. In the "cmp" mode, it corresponds queries user entered
	// for comparison.
	QueryIdx int `json:"queryIdx"`

	/**
	 * OtherTileID, if specified, declares that the tile
	 * TileID wants to read the same data as OtherTileID.
	 * If set, then URL, Method, Body are ignored and
	 * APIGuard will respond by the corresponding data
	 * also to this tile.
	 *
	 */
	OtherTileID *int `json:"otherTileId"`

	// OtherTileQueryIdx has the same function as QueryIdx
	// but for the "other" tile (i.e. to address a concrete
	// query in the "other" tile)
	OtherTileQueryIdx *int `json:"otherTileQueryIdx"`

	// URL is an APIGuard service URL we want to query
	// and read the result from data stream. I.e. this
	// cannot be a backend service URL but rather the
	// url handled by apiguard. E.g.:
	// orig API URL: http://192.168.1.50:8080/foo
	// apiguard URL: http://192.168.1.30:8000/service/0/foo
	// ... here we must use the latter variant as we need
	// to be able to process the URL via Gin's router.
	URL string `json:"url"`

	// Method is the HTTP method of the backend API
	Method string `json:"method"`

	// Body (optional) is an HTTP request body
	Body string `json:"body"`

	// ContentType specifies data type of request's body and
	// is passed to a respective API
	ContentType string `json:"contentType"`

	// Base64EncodeResult - if true, then APIGuard will encode any incoming
	// data for the service into base64. This may be needed in case data may
	// be in conflict with the EventSource data formatting (e.g. if a data
	// source returns HTML data).
	Base64EncodeResult bool `json:"base64EncodeResult"`

	// IsEventSource allows us to integrate API which itself provides
	// its data as an EventSource stream. In such case, the API must
	// be able to configure a proper `event` entry so the reader can
	// identify which data chunks belongs to the required response.
	IsEventSource bool `json:"isEventSource"`
}

func (req *request) groupingKey() string {
	var tileID int
	if req.OtherTileID != nil {
		tileID = *req.OtherTileID

	} else {
		tileID = req.TileID
	}
	return fmt.Sprintf("%d.%d", tileID, req.QueryIdx)
}

// --------------------------------------------------

// StreamRequestJSON represents an HTTP body of a request
// to APIGuard's data streaming API proxy.
type StreamRequestJSON struct {
	Requests []*request `json:"requests"`
}

func (srj *StreamRequestJSON) ApplyDefaults() {
	for _, v := range srj.Requests {
		if v.Method == "" {
			v.Method = "GET"
		}
		if v.ContentType == "" {
			v.ContentType = "application/json"
		}
	}
}
