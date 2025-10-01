// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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

package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func GetCookieValue(req *http.Request, cookieName string) string {
	var cookieValue string
	for _, cookie := range req.Cookies() {
		if cookie.Name == cookieName {
			cookieValue = cookie.Value
			break
		}
	}
	return cookieValue
}

// LogCookies logs all the cookies found in provided request
// using zerolog's Str() function. The names of cookies have
// added 'cookie_' prefixes for easier distinction.
// The function returns the same target as the provided one
// for convenient function chaining.
func LogCookies(req *http.Request, target *zerolog.Event) *zerolog.Event {
	for _, cookie := range req.Cookies() {
		target.Str(fmt.Sprintf("cookie_%s", cookie.Name), cookie.Value)
	}
	return target
}

// CoreProxy is a minimum common functionality needed
// to proxy requests to APIGuard to different backends/APIs.
// It has only methods for performing request and request stream
// (Server-side events).
type CoreProxy struct {
	BackendURL  *url.URL
	FrontendURL *url.URL
	client      *http.Client
}

func (proxy *CoreProxy) transformRedirect(headers http.Header) error {
	for name, vals := range headers {
		if name == "Location" {
			redirectURL, err := url.Parse(vals[0])
			if err != nil {
				return err
			}
			// situations like this:
			// APIGuard provides access to KonText via http://localhost:3010/services/kontext
			// External KonText API URL is https://www.korpus.cz/kontext-api/v0.17
			// Now KonText wants to redirect to https://localhost:8195/kontext-api/v0.17/query
			// => we have to replace Host
			if redirectURL.Host == proxy.BackendURL.Host {
				redirectURL.Host = proxy.FrontendURL.Host
			}
			headers[name] = []string{redirectURL.String()}
			break
		}
	}
	return nil
}

func (proxy *CoreProxy) Request(
	urlPath string,
	args url.Values,
	method string,
	headers http.Header,
	rbody io.Reader,
) *BackendProxiedResponse {

	targetURL := proxy.BackendURL.JoinPath(urlPath)
	targetURL.RawQuery = args.Encode()
	req, err := http.NewRequest(method, targetURL.String(), rbody)
	if err != nil {
		return &BackendProxiedResponse{
			BodyReader: EmptyReadCloser{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	req.Header = headers
	resp, err := proxy.client.Do(req)
	if err != nil {
		return &BackendProxiedResponse{
			BodyReader: EmptyReadCloser{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	log.Debug().
		Str("url", targetURL.String()).
		Err(err).
		Int("status", resp.StatusCode).
		Msgf(">>> Proxy request >>>")

	ansHeaders := resp.Header
	proxy.transformRedirect(ansHeaders)
	return &BackendProxiedResponse{
		BodyReader: resp.Body,
		Headers:    ansHeaders,
		StatusCode: resp.StatusCode,
		Err:        nil,
	}
}

func (proxy *CoreProxy) RequestStream(
	urlPath string,
	args url.Values,
	method string,
	headers http.Header,
	rbody io.Reader,
) *BackendProxiedStreamResponse {

	targetURL := proxy.BackendURL.JoinPath(urlPath)
	targetURL.RawQuery = args.Encode()
	req, err := http.NewRequest(method, targetURL.String(), rbody)
	if err != nil {
		return &BackendProxiedStreamResponse{
			BodyReader: EmptyReadCloser{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	req.Header = headers
	resp, err := proxy.client.Do(req)
	if err != nil {
		return &BackendProxiedStreamResponse{
			BodyReader: EmptyReadCloser{},
			Headers:    http.Header{},
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	log.Debug().
		Str("url", targetURL.String()).
		Err(err).
		Int("status", resp.StatusCode).
		Msgf(">>> Proxy request >>>")

	ansHeaders := resp.Header
	proxy.transformRedirect(ansHeaders)
	return &BackendProxiedStreamResponse{
		BodyReader: resp.Body,
		Headers:    ansHeaders,
		StatusCode: resp.StatusCode,
		Err:        nil,
	}
}

func NewCoreProxy(conf GeneralProxyConf) (*CoreProxy, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = httpclient.TransportMaxIdleConns
	transport.MaxConnsPerHost = httpclient.TransportMaxConnsPerHost
	transport.MaxIdleConnsPerHost = httpclient.TransportMaxIdleConnsPerHost
	transport.IdleConnTimeout = time.Duration(conf.IdleConnTimeoutSecs) * time.Second
	backendURL, err := url.Parse(conf.BackendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create APIProxy: %w", err)
	}
	frontendURL, err := url.Parse(conf.FrontendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create APIProxy: %w", err)
	}
	return &CoreProxy{
		BackendURL:  backendURL,
		FrontendURL: frontendURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout:   time.Duration(conf.ReqTimeoutSecs) * time.Second,
			Transport: transport,
		},
	}, nil
}
