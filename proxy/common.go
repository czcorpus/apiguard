// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"net/http"
	"strings"
	"time"

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
	GetBody() []byte
	GetHeaders() http.Header
	GetStatusCode() int
	IsCached() bool
	MarkCached()
	GetError() error
}

type CacheEntryOptions struct {
	respectCookies []string
	requestBody    []byte
	cacheablePOST  bool
}

func CachingWithCookies(cookies []string) func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.respectCookies = cookies
	}
}

func CachingWithReqBody(body []byte) func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.requestBody = body
	}
}

func CachingWithCacheablePOST() func(*CacheEntryOptions) {
	return func(opts *CacheEntryOptions) {
		opts.cacheablePOST = true
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
