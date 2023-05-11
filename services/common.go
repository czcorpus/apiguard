// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"net/http"
	"time"
)

type GeneralProxyConf struct {
	InternalURL         string
	ExternalURL         string
	ReqTimeoutSecs      int
	IdleConnTimeoutSecs int
}

type ProxyProcReport struct {
	DateTime time.Time
	ProcTime float32 `json:"procTime"`
	Status   int     `json:"status"`
	Service  string  `json:"service"`
}

func (report ProxyProcReport) ToInfluxDB() (map[string]string, map[string]any) {
	return map[string]string{
			"service": report.Service,
		},
		map[string]any{
			"procTime": report.ProcTime,
			"status":   report.Status,
		}
}

func (report ProxyProcReport) GetTime() time.Time {
	return report.DateTime
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
