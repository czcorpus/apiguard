// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kla

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "kla"
)

type KLAActions struct {
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	guard           guard.ServiceGuard
}

type Response struct {
	Images []string `json:"images"`
	Query  string   `json:"query"`
}

func (aa *KLAActions) Query(ctx *gin.Context) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
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

	queries, ok := ctx.Request.URL.Query()["q"]
	if !ok {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}
	if len(queries) != 1 && len(queries) > aa.conf.MaxQueries {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("too many queries"), 422)
		return
	}
	maxImages := ctx.Request.URL.Query().Get("maxImages")
	if maxImages == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty maxImages"), 422)
		return
	}
	maxImageCount, err := strconv.Atoi(maxImages)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err = guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard, clientID)
	if err != nil {
		return
	}

	var images []string
	var query string
	for _, query = range queries {
		resp := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledej&heslo=%s&where=hesla&zobraz_cards=cards&pocet_karet=100&not_initial=1", aa.conf.BaseURL, url.QueryEscape(query)),
			ctx.Request,
		)
		cached = cached || resp.IsCacheHit()
		body, err := resp.ExportResponse()
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, 500)
			return
		}
		images, err = parseData(string(body), maxImageCount, aa.conf.BaseURL)
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		if len(images) > 0 {
			break
		}
	}

	uniresp.WriteJSONResponse(ctx.Writer, Response{
		Images: images,
		Query:  query,
	})
}

func (aa *KLAActions) createMainRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, aa.conf.ClientUserAgent, aa.globalCtx.Cache)
}

func NewKLAActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	guard guard.ServiceGuard,
	readTimeoutSecs int,
) *KLAActions {
	return &KLAActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
