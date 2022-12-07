// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

func GetSessionKey(req *http.Request, cookieName string) string {
	var cookieValue string
	for _, cookie := range req.Cookies() {
		if cookie.Name == cookieName {
			cookieValue = cookie.Value
			break
		}
	}
	return cookieValue
}

func GetRequest(url, userAgent string) (string, int, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	sbody := string(body)
	return sbody, resp.StatusCode, nil
}

type ProxiedResponse struct {
	Body       []byte
	Headers    http.Header
	StatusCode int
	Err        error
}

type APIProxy struct {
	InternalURL string
	ExternalURL string
}

func (proxy *APIProxy) transformRedirect(headers http.Header) error {
	for name, vals := range headers {
		if name == "Location" {
			var err error
			internalURL, err := url.Parse(proxy.InternalURL)
			if err != nil {
				return err
			}
			externalURL, err := url.Parse(proxy.ExternalURL)
			if err != nil {
				return err
			}
			redirectURL, err := url.Parse(vals[0])
			if err != nil {
				return err
			}
			// situations like this:
			// APIGuard provides access to KonText via http://localhost:3010/services/kontext
			// External KonText API URL is https://www.korpus.cz/kontext-api/v0.17
			// Now KonText wants to redirect to https://localhost:8195/kontext-api/v0.17/query
			// => we have to replace Host
			if redirectURL.Host == internalURL.Host {
				redirectURL.Host = externalURL.Host
			}
			headers[name] = []string{redirectURL.String()}
			break
		}
	}
	return nil
}

func (proxy *APIProxy) Request(urlPath, method string, headers http.Header, rbody io.Reader) *ProxiedResponse {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", proxy.InternalURL, urlPath), rbody)
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
	ansHeaders := resp.Header
	proxy.transformRedirect(ansHeaders)
	return &ProxiedResponse{
		Body:       body,
		Headers:    ansHeaders,
		StatusCode: resp.StatusCode,
		Err:        nil,
	}
}
