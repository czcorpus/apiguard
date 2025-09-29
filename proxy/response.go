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

package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/czcorpus/apiguard-common/cache"
	"github.com/czcorpus/apiguard-common/proxy"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

func isCacheableStatusCode(code int) bool {
	return 200 <= code && code < 300
}

// -------

// CachedResponse handles response delivery using pre-cached data from storage.
type CachedResponse struct {
	status  int
	headers http.Header
	data    []byte
}

func (cw *CachedResponse) String() string {
	headers := http.Header{}
	if cw.headers != nil {
		headers = cw.headers
	}
	return fmt.Sprintf(
		"CachedResponse{status: %d, content-type: %s, data len: %d}",
		cw.status, headers.Get("content-type"), len(cw.data),
	)
}

func (cw *CachedResponse) WriteResponse(w http.ResponseWriter) {
	for hdName, hdValue := range cw.headers {
		w.Header().Set(hdName, hdValue[len(hdValue)-1])
	}
	w.Write(cw.data)
}

func (cw *CachedResponse) ExportResponse() ([]byte, error) {
	return cw.data, nil
}

func (cw *CachedResponse) Response() proxy.BackendResponse {
	if strings.Contains(cw.headers.Get("Content-Type"), "text/event-stream") {
		return &BackendProxiedStreamResponse{
			BodyReader: io.NopCloser(bytes.NewBuffer(cw.data)),
		}
	}
	return &proxy.BackendSimpleResponse{
		BodyReader: io.NopCloser(bytes.NewBuffer(cw.data)),
	}
}

func (cw *CachedResponse) Error() error {
	return nil
}

func (cw *CachedResponse) IsCacheHit() bool {
	return true
}

func (cw *CachedResponse) HandleCacheMiss(func() proxy.BackendResponse) {
	// NO-OP
}

func NewCachedResponse(status int, headers http.Header, data []byte) *CachedResponse {
	return &CachedResponse{
		status:  status,
		headers: headers,
		data:    data,
	}
}

// ---------------------------

// ThroughCacheResponse handles response delivery with cache-through behavior,
// storing response data to cache while simultaneously writing to client.
type ThroughCacheResponse struct {
	error     error
	req       *http.Request
	cache     cache.Cache
	boundResp proxy.BackendResponse
	opts      []func(*cache.CacheEntryOptions)
}

func (ncw *ThroughCacheResponse) String() string {
	isDataStream := ncw.boundResp != nil && ncw.boundResp.IsDataStream()
	reqUrl := "??"
	if ncw.req != nil {
		reqUrl = ncw.req.URL.String()
	}
	return fmt.Sprintf(
		"ThroughCacheResponse{req: %s, bound: %t, isDataStream: %t}",
		reqUrl, ncw.boundResp != nil, isDataStream,
	)
}

func (ncw *ThroughCacheResponse) ExportResponse() ([]byte, error) {
	if ncw.boundResp == nil {
		return nil, fmt.Errorf("cannot export response - no BackendResponse bound")
	}
	defer ncw.boundResp.GetBodyReader().Close()
	data, err := io.ReadAll(ncw.boundResp.GetBodyReader())
	if err != nil {
		return nil, fmt.Errorf("failed to export response from DirectResponse: %w", err)
	}
	return data, nil
}

func (ncw *ThroughCacheResponse) writeSSEResponse(w http.ResponseWriter) {
	defer ncw.boundResp.CloseBodyReader()
	scanner := bufio.NewScanner(ncw.boundResp.GetBodyReader())
	var eventChunk []string
	toCache := new(bytes.Buffer)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(eventChunk) > 0 {
				completeEvent := strings.Join(eventChunk, "\n") + "\n\n"
				tmp := []byte(completeEvent)
				toCache.Write(tmp)
				w.Write(tmp)
				eventChunk = eventChunk[:0]
			}

		} else {
			eventChunk = append(eventChunk, line)
		}
	}
	if len(eventChunk) > 0 {
		completeEvent := strings.Join(eventChunk, "\n") + "\n\n"
		tmp := []byte(completeEvent)
		toCache.Write(tmp)
		w.Write(tmp)
	}
	if isCacheableStatusCode(ncw.boundResp.GetStatusCode()) {
		ncw.cache.Set(
			ncw.req,
			cache.CacheEntry{
				Status:  http.StatusOK,
				Data:    toCache.Bytes(),
				Headers: ncw.boundResp.GetHeaders(),
			},
		)
	}
}

// WriteResponse
func (ncw *ThroughCacheResponse) WriteResponse(w http.ResponseWriter) {

	for k, v := range ncw.boundResp.GetHeaders() {
		w.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	w.WriteHeader(ncw.boundResp.GetStatusCode())
	defer ncw.boundResp.CloseBodyReader()

	if ncw.boundResp.IsDataStream() {
		ncw.writeSSEResponse(w)

	} else {
		data, err := io.ReadAll(ncw.boundResp.GetBodyReader())
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
			return
		}
		if isCacheableStatusCode(ncw.boundResp.GetStatusCode()) {
			if err := ncw.cache.Set(
				ncw.req,
				cache.CacheEntry{
					Status:  http.StatusOK,
					Data:    data,
					Headers: w.Header(),
				},
				ncw.opts...); err != nil {
				log.Error().Err(err).Msg("failed to cache response")
			}
		}
		w.Write(data)
	}
}

func (ncw *ThroughCacheResponse) Response() proxy.BackendResponse {
	if ncw.boundResp != nil {
		return ncw.boundResp
	}
	return &proxy.BackendZeroResponse{}
}

// Error returns any error that occurred
// while retrieving the cache value.
// This excludes CacheMiss errors but includes
// errors from the bound response (if present).
func (ncw *ThroughCacheResponse) Error() error {
	if ncw.error != nil {
		return ncw.error
	}
	if ncw.boundResp != nil && ncw.boundResp.Error() != nil {
		return ncw.boundResp.Error()
	}
	return nil
}

func (ncw *ThroughCacheResponse) IsCacheHit() bool {
	return false
}

func (ncw *ThroughCacheResponse) HandleCacheMiss(fn func() proxy.BackendResponse) {
	ncw.boundResp = fn()
}

func NewThroughCacheResponse(req *http.Request, cache cache.Cache, err error) *ThroughCacheResponse {
	return &ThroughCacheResponse{
		req:   req,
		cache: cache,
		error: err,
	}
}
