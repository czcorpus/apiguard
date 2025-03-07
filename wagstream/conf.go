// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

// request is a single API request which we pack into
// an HTTP stream request.
type request struct {
	TileID int `json:"tileId"`

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
}

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
