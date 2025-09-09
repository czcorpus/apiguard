// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cja

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
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
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	guard           guard.ServiceGuard
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
		aa.globalCtx.BackendLoggers.Get(aa.serviceKey).Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			common.InvalidUserID,
			false,
			reporting.BackendActionTypeQuery,
		)
	}()

	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard, clientID)
	if err != nil {
		return
	}
	resp := aa.createRequests(
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		ctx.Request,
	)
	if resp.Error() != nil {
		uniresp.RespondWithErrorJSON(ctx, resp.Error(), 500)
		return
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, 500)
		return
	}
	response, err := parseData(string(respBody), aa.conf.BaseURL)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, 500)
		return
	}
	// TODO !!!! response.Backlink = backlink

	uniresp.WriteJSONResponse(ctx.Writer, response)
}

func (aa *CJAActions) createSubRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, req.Header.Get("user-agent"), aa.globalCtx.Cache)
}

func (aa *CJAActions) createRequests(url1 string, url2 string, req *http.Request) proxy.ResponseProcessor {
	resp := aa.createSubRequest(url1, req)
	if resp.Error() != nil {
		return resp
	}
	if resp.Response().GetStatusCode() == 500 {
		return aa.createSubRequest(url2, req)
	}
	return resp
}

func NewCJAActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	guard guard.ServiceGuard,
	readTimeoutSecs int,
) *CJAActions {
	return &CJAActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
