// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neomat

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/reqcache"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "neomat"
)

type NeomatActions struct {
	globalCtx       *globctx.Context
	conf            *Conf
	readTimeoutSecs int
	analyzer        guard.ServiceGuard
}

type Response struct {
	Entries []string `json:"entries"`
}

func (aa *NeomatActions) Query(ctx *gin.Context) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		aa.globalCtx.BackendLogger.Log(
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
	maxItems := ctx.Request.URL.Query().Get("maxItems")
	if maxItems == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty maxItems"), 422)
		return
	}
	maxItemsCount, err := strconv.Atoi(maxItems)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err = guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.analyzer, clientID)
	if err != nil {
		return
	}
	resp := aa.createMainRequest(
		fmt.Sprintf("%s/index.php?retezec=%s&prijimam=1", aa.conf.BaseURL, url.QueryEscape(query)),
		ctx.Request,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	entries, err := parseData(string(resp.GetBody()), maxItemsCount)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	uniresp.WriteJSONResponse(ctx.Writer, Response{Entries: entries})
}

func (aa *NeomatActions) createMainRequest(url string, req *http.Request) proxy.BackendResponse {
	resp, err := aa.globalCtx.Cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = proxy.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.globalCtx.Cache.Set(req, resp, nil)
		if err != nil {
			return &proxy.SimpleResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &proxy.SimpleResponse{Err: err}
	}
	return resp
}

func NewNeomatActions(
	globalCtx *globctx.Context,
	conf *Conf,
	analyzer guard.ServiceGuard,
	readTimeoutSecs int,
) *NeomatActions {
	return &NeomatActions{
		globalCtx:       globalCtx,
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
