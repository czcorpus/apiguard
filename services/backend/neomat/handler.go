// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neomat

import (
	"apiguard/botwatch"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

const (
	ServiceName = "neomat"
)

type NeomatActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Entries []string `json:"entries"`
}

func (aa *NeomatActions) Query(w http.ResponseWriter, req *http.Request) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		aa.globalCtx.BackendLogger.Log(
			req, ServiceName, time.Since(t0), cached, common.InvalidUserID, false)
	}()

	query := req.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("empty query"), 422)
		return
	}
	maxItems := req.URL.Query().Get("maxItems")
	if maxItems == "" {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("empty maxItems"), 422)
		return
	}
	maxItemsCount, err := strconv.Atoi(maxItems)
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
		return
	}

	err = services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}
	resp := aa.createMainRequest(
		fmt.Sprintf("%s/index.php?retezec=%s&prijimam=1", aa.conf.BaseURL, url.QueryEscape(query)),
		req,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
		return
	}

	entries, err := parseData(string(resp.GetBody()), maxItemsCount)
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
		return
	}

	uniresp.WriteJSONResponse(w, Response{Entries: entries})
}

func (aa *NeomatActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.cache.Set(req, resp, nil)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &services.SimpleResponse{Err: err}
	}
	return resp
}

func NewNeomatActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *NeomatActions {
	return &NeomatActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
