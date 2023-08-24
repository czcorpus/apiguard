// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kla

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
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "kla"
)

type KLAActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Images []string `json:"images"`
	Query  string   `json:"query"`
}

func (aa *KLAActions) Query(ctx *gin.Context) {
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

	err = services.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.analyzer)
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
		cached = cached || resp.IsCached()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}

		images, err = parseData(string(resp.GetBody()), maxImageCount, aa.conf.BaseURL)
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

func (aa *KLAActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.cache.Set(req, resp, nil)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}

	} else if err != nil {
		return &services.SimpleResponse{Err: err}
	}
	return resp
}

func NewKLAActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *KLAActions {
	return &KLAActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
