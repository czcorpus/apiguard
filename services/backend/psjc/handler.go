// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package psjc

import (
	"apiguard/botwatch"
	"apiguard/ctx"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

const (
	ServiceName = "psjc"
)

type PSJCActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Entries []string `json:"entries"`
	Query   string   `json:"query"`
}

func (aa *PSJCActions) Query(w http.ResponseWriter, req *http.Request) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		aa.globalCtx.BackendLogger.Log(ServiceName, time.Since(t0), &cached, nil)
	}()

	queries, ok := req.URL.Query()["q"]
	if !ok {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("empty query"), 422)
		return
	}
	if len(queries) != 1 && len(queries) > aa.conf.MaxQueries {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("too many queries"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}

	var entries []string
	var query string
	for _, query = range queries {
		resp := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledej&heslo=%s&where=hesla&zobraz_ps=ps&not_initial=1", aa.conf.BaseURL, url.QueryEscape(query)),
			req,
		)
		cached = cached || resp.IsCached()
		if err != nil {
			uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
			return
		}

		entries, err = parseData(string(resp.GetBody()))
		if err != nil {
			uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
			return
		}

		if len(entries) > 0 {
			break
		}
	}

	uniresp.WriteJSONResponse(w, Response{
		Entries: entries,
		Query:   query,
	})
}

func (aa *PSJCActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp := services.GetRequest(url, aa.conf.ClientUserAgent)
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

func NewPSJCActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *PSJCActions {
	return &PSJCActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
