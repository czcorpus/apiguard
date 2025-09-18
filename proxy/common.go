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
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type Limit struct {
	ReqPerTimeThreshold     int `json:"reqPerTimeThreshold"`
	ReqCheckingIntervalSecs int `json:"reqCheckingIntervalSecs"`
	BurstLimit              int `json:"burstLimit"`
}

func (m Limit) ReqCheckingInterval() time.Duration {
	return time.Duration(m.ReqCheckingIntervalSecs) * time.Second
}

func (m Limit) NormLimitPerSec() rate.Limit {
	return rate.Limit(float64(m.ReqPerTimeThreshold) / float64(m.ReqCheckingIntervalSecs))
}

// --------------------------

type GeneralProxyConf struct {
	BackendURL          string
	FrontendURL         string
	ReqTimeoutSecs      int
	IdleConnTimeoutSecs int
	Limits              []Limit
}

// ---------------------------

type BackendResponse interface {
	GetBodyReader() io.ReadCloser
	CloseBodyReader() error
	GetHeaders() http.Header
	GetStatusCode() int
	IsDataStream() bool
	Error() error
}

// ---------------------------

type CacheEntryOptions struct {
	RespectCookies []string
	RequestBody    []byte
	CacheablePOST  bool

	// tag may serve for debugging/reviewing cached entries
	Tag string
}

func CachingWithCookies(cookies []string) func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.RespectCookies = cookies
	}
}

func CachingWithReqBody(body []byte) func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.RequestBody = body
	}
}

// CachingWithCacheable POST will allow for caching post requests.
// It will also include POST request body to generate a cache entry key.
// Please note this should be used only for requests which are really
// GET-like requests (i.e. they do not change resources on the server side
// and POST is used only because the args cannot fit from one reason on
// another to URL query).
func CachingWithCacheablePOST() func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.CacheablePOST = true
	}
}

// CachingWithTag sets a tag which may become part
// of cache's record. Some backend may not support it,
// in which case they should silently ignore the option.
func CachingWithTag(tag string) func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.Tag = tag
	}
}

// ------------------------------

type CacheEntry struct {
	Status  int
	Data    []byte
	Headers http.Header
}

func (ce CacheEntry) IsZero() bool {
	return ce.Status == 0
}

// -----------------------------

type Cache interface {
	Get(req *http.Request, opts ...func(*CacheEntryOptions)) (CacheEntry, error)
	Set(req *http.Request, value CacheEntry, opts ...func(*CacheEntryOptions)) error
}

// -----------------------------

type GlobalContext struct {
	TimezoneLocation *time.Location
}

// -----------------------------

// ExtractClientIP
// Deprecated: Gin offers a better solution for this
func ExtractClientIP(req *http.Request) string {
	ip := req.Header.Get("x-forwarded-for")
	if ip != "" {
		return strings.Split(ip, ",")[0]
	}
	ip = req.Header.Get("x-real-ip")
	if ip != "" {
		return ip
	}
	return strings.Split(req.RemoteAddr, ":")[0]
}

// -------------------------------

func WriteError(ctx *gin.Context, err error, status int) {
	if ctx.Request.Header.Get("content-type") == "application/json" ||
		ctx.Request.Header.Get("content-type") == "text/event-stream" {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to proxy request: %s", err),
			status,
		)

	} else {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to proxy request: %s", err),
			status,
		)
	}
}

// --------------------------------

type MultiStatusCode []int

// Result provides a single HTTP status code
// based on particular status codes of some sub-requests
func (msc MultiStatusCode) Result() int {
	var maxCode int
	var has200 bool
	for _, v := range msc {
		if v == http.StatusOK {
			has200 = true
		}
		if v > maxCode {
			maxCode = v
		}
	}
	if has200 && maxCode < 500 {
		return http.StatusOK
	}
	if maxCode >= 500 {
		return http.StatusBadGateway
	}
	return maxCode
}
