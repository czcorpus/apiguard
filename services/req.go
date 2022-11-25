// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
)

func GetRequest(url, userAgent string) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	sbody := string(body)
	return sbody, nil
}

type ProxiedResponse struct {
	Body       []byte
	Headers    http.Header
	StatusCode int
	Err        error
}

func ProxiedRequest(url, method string, headers http.Header) *ProxiedResponse {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return &ProxiedResponse{
			Body:       []byte{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	req.Header = headers
	resp, err := client.Do(req)
	if err != nil {
		return &ProxiedResponse{
			Body:       []byte{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &ProxiedResponse{
			Body:       []byte{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	return &ProxiedResponse{
		Body:       body,
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		Err:        nil,
	}
}
