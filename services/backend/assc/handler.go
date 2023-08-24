// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

import (
	"apiguard/botwatch"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/monitoring"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

/*
note:
curl
	--header "Origin: https://slovnikcestiny.cz"
	--header "Referer: https://slovnikcestiny.cz/heslo/batalion/0/548"
	--header "X-Requested-With: XMLHttpRequest" --header "Host: slovnikcestiny.cz"
	https://slovnikcestiny.cz/web_ver_ajax.php
*/

const (
	ServiceName = "assc"
)

type ASSCActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	analyzer        *botwatch.Analyzer
}

func (aa *ASSCActions) Query(ctx *gin.Context) {
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
			monitoring.BackendActionTypeQuery,
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

	err := services.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}

	var data *dataStruct
	for _, query := range queries {
		response := aa.createMainRequest(
			fmt.Sprintf("%s/heslo/%s/", aa.conf.BaseURL, url.QueryEscape(query)),
			ctx.Request,
		)
		cached = cached || response.IsCached()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		data, err = parseData(string(response.GetBody()))
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		// check if result is not empty and contains query key
		if data.lastItem != nil {
			data.Query = query
			break
		}
	}
	uniresp.WriteJSONResponse(ctx.Writer, data)
}

func (aa *ASSCActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.globalCtx.Cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.globalCtx.Cache.Set(req, resp, nil)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}

	} else if err != nil {
		return &services.SimpleResponse{Err: err}
	}
	return resp
}

func NewASSCActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *ASSCActions {
	return &ASSCActions{
		globalCtx:       globalCtx,
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
