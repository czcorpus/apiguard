// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"net/http"
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

type Cache interface {
	Get(req *http.Request, respectCookies []string) (BackendResponse, error)
	Set(req *http.Request, resp BackendResponse, respectCookies []string) error
}

type GlobalContext struct {
	TimezoneLocation *time.Location
}
