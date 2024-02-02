// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cja

import (
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/guard/telemetry"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/reqcache"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "cja"
)

type CJAActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	guard           *telemetry.Guard
}

type Response struct {
	Content  string `json:"content"`
	Image    string `json:"image"`
	CSS      string `json:"css"`
	Backlink string `json:"backlink"`
}

func (aa *CJAActions) Query(ctx *gin.Context) {
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	var cached bool
	defer func() {
		aa.globalCtx.BackendLogger.Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			common.InvalidUserID,
			false,
			monitoring.BackendActionTypeQuery,
		)
	}()

	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}

	err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard)
	if err != nil {
		return
	}
	resp := aa.createRequests(
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		ctx.Request,
	)
	if resp.GetError() != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	response, err := parseData(string(resp.GetBody()), aa.conf.BaseURL)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}
	// TODO !!!! response.Backlink = backlink

	uniresp.WriteJSONResponse(ctx.Writer, response)
}

func (aa *CJAActions) createSubRequest(url string, req *http.Request) proxy.BackendResponse {
	resp, err := aa.globalCtx.Cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = proxy.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.globalCtx.Cache.Set(req, resp, nil)
		if err != nil {
			return &proxy.SimpleResponse{Err: err}
		}
	}
	return resp
}

func (aa *CJAActions) createRequests(url1 string, url2 string, req *http.Request) proxy.BackendResponse {
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
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	guard *telemetry.Guard,
	readTimeoutSecs int,
) *CJAActions {
	return &CJAActions{
		globalCtx:       globalCtx,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
