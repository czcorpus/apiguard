// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

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

type GeneralProxyConf struct {
	BackendURL          string
	FrontendURL         string
	ReqTimeoutSecs      int
	IdleConnTimeoutSecs int
	Limits              []Limit
}

type BackendResponse interface {
	GetBodyReader() io.ReadCloser
	CloseBodyReader() error
	GetHeaders() http.Header
	GetStatusCode() int
	IsDataStream() bool
	IsCached() bool
	MarkCached()
	GetError() error
}

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

type Cache interface {
	Get(req *http.Request, opts ...func(*CacheEntryOptions)) (BackendResponse, error)
	Set(req *http.Request, resp BackendResponse, opts ...func(*CacheEntryOptions)) error
}

type GlobalContext struct {
	TimezoneLocation *time.Location
}

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
