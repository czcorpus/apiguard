// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"crypto/tls"
	"net/http"
)

func UJCGetRequest(url, userAgent string, cache Cache) ResponseProcessor {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &DirectResponse{boundResp: &BackendSimpleResponse{Err: err}}
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return &DirectResponse{boundResp: &BackendSimpleResponse{Err: err}}
	}
	return &DirectResponse{
		boundResp: &BackendSimpleResponse{
			StatusCode: resp.StatusCode,
			BodyReader: resp.Body,
		},
	}
}
