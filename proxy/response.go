// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

// ResponseProcessor is an abstraction for handling cache-aware response processing.
type ResponseProcessor interface {
	Response() BackendResponse
	Error() error
	IsCacheMiss() bool
	WriteResponse(w http.ResponseWriter)
	ExportResponse() ([]byte, error)
}

// ResponseProcessorBinder specifies a general response processor
// which allows for attaching a backend response to itself. This is
// typically used on cache miss, when we need first send a request
// to a backend and attach the raw respose to a generalized response
// object for later use.
type ResponseProcessorBinder interface {

	// BindResponse attaches the response from a backend
	// to the value
	BindResponse(resp BackendResponse)
}

// -----

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

func (cw *CachedResponse) Response() BackendResponse {
	if strings.Contains(cw.headers.Get("Content-Type"), "text/event-stream") {
		return &BackendProxiedStreamResponse{
			BodyReader: io.NopCloser(bytes.NewBuffer(cw.data)),
		}
	}
	return &BackendSimpleResponse{
		BodyReader: io.NopCloser(bytes.NewBuffer(cw.data)),
	}
}

func (cw *CachedResponse) Error() error {
	return nil
}

func (cw *CachedResponse) IsCacheMiss() bool {
	return false
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
	cache     Cache
	boundResp BackendResponse
	opts      []func(*CacheEntryOptions)
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
			CacheEntry{
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
				CacheEntry{
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

func (ncw *ThroughCacheResponse) Response() BackendResponse {
	if ncw.boundResp != nil {
		return ncw.boundResp
	}
	return &BackendZeroResponse{}
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

func (ncw *ThroughCacheResponse) IsCacheMiss() bool {
	return true
}

func (ncw *ThroughCacheResponse) BindResponse(resp BackendResponse) {
	ncw.boundResp = resp
}

func NewThroughCacheResponse(req *http.Request, cache Cache, err error) *ThroughCacheResponse {
	return &ThroughCacheResponse{
		req:   req,
		cache: cache,
		error: err,
	}
}

// ----------------------------------------------------

// DirectResponse handles response delivery by bypassing cache entirely,
// writing response data directly to client without caching.
type DirectResponse struct {
	error     error
	boundResp BackendResponse
}

func (ncw *DirectResponse) String() string {
	isDataStream := ncw.boundResp != nil && ncw.boundResp.IsDataStream()
	return fmt.Sprintf(
		"DirectResponse{err: %s, bound: %t, isDataStream: %t}",
		ncw.error, ncw.boundResp != nil, isDataStream,
	)
}

func (ncw *DirectResponse) ExportResponse() ([]byte, error) {
	data, err := io.ReadAll(ncw.boundResp.GetBodyReader())
	if err != nil {
		return nil, fmt.Errorf("failed to export response from DirectResponse: %w", err)
	}
	return data, nil
}

// DirectResponse
func (ncw *DirectResponse) WriteResponse(w http.ResponseWriter) {
	data, err := io.ReadAll(ncw.boundResp.GetBodyReader())
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	jsonAns, err := json.Marshal(data)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteRawJSONResponse(w, jsonAns)
}

func (ncw *DirectResponse) Response() BackendResponse {
	if ncw.boundResp != nil {
		return ncw.boundResp
	}
	return &BackendZeroResponse{}
}

// Error returns any error that occurred
// while retrieving the cache value.
// This excludes CacheMiss errors but includes
// errors from the bound response (if present).
func (ncw *DirectResponse) Error() error {
	if ncw.error != nil {
		return ncw.error
	}
	if ncw.boundResp != nil && ncw.boundResp.Error() != nil {
		return ncw.boundResp.Error()
	}
	return nil
}

func (ncw *DirectResponse) IsCacheMiss() bool {
	return true
}

func (ncw *DirectResponse) BindResponse(resp BackendResponse) {
	ncw.boundResp = resp
}

func NewDirectResponse(err error) *DirectResponse {
	return &DirectResponse{
		error: err,
	}
}
