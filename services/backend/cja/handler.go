// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cja

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	ServiceName = "cja"
)

type CJAActions struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Content  string `json:"content"`
	Image    string `json:"image"`
	CSS      string `json:"css"`
	Backlink string `json:"backlink"`
}

func (aa *CJAActions) Query(w http.ResponseWriter, req *http.Request) {
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	var cached bool
	defer func() {
		services.LogServiceRequest(ServiceName, t0, &cached, nil)
	}()

	query := req.URL.Query().Get("q")
	if query == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty query"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}
	resp := aa.createRequests(
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		req,
	)
	if resp.GetError() != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	response, err := parseData(string(resp.GetBody()), aa.conf.BaseURL)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	// TODO !!!! response.Backlink = backlink

	services.WriteJSONResponse(w, response)
}

func (aa *CJAActions) createSubRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.cache.Set(req, resp)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}
	}
	return resp
}

func (aa *CJAActions) createRequests(url1 string, url2 string, req *http.Request) services.BackendResponse {
	resp := aa.createSubRequest(url1, req)
	if resp.GetError() != nil {
		return resp
	}
	if resp.GetStatusCode() == 500 {
		return aa.createSubRequest(url2, req)
	}
	return resp
}

func NewCJAActions(
	globalCtx *services.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *CJAActions {
	return &CJAActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
