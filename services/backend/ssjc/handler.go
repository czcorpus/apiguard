// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ssjc

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

const (
	ServiceName = "ssjc"
)

type SSJCActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	analyzer        *botwatch.Analyzer
}

type Entry struct {
	STI     *int   `json:"sti"`
	Payload string `json:"payload"`
}

type Response struct {
	Entries []Entry `json:"entries"`
	Query   string  `json:"query"`
}

func (aa *SSJCActions) Query(ctx *gin.Context) {
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

	response := Response{Entries: make([]Entry, 0)}
	var query string
	for _, query = range queries {
		resp := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledat&heslo=%s&where=hesla&hsubstr=no", aa.conf.BaseURL, url.QueryEscape(query)),
			ctx.Request,
		)
		cached = cached || resp.IsCached()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}

		// check if there are multiple results
		STIs, err := lookForSTI(string(resp.GetBody()))
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}

		if len(STIs) > 0 {
			for _, STI := range STIs {
				resp := aa.createMainRequest(
					fmt.Sprintf("%s/search.php?hledej=Hledat&sti=%d&where=hesla&hsubstr=no", aa.conf.BaseURL, STI),
					ctx.Request,
				)
				cached = cached || resp.IsCached()
				if err != nil {
					uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
					return
				}
				payload, err := parseData(string(resp.GetBody()))
				if err != nil {
					uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
					return
				}
				response.Entries = append(
					response.Entries,
					Entry{STI: &STI, Payload: payload},
				)
			}
			response.Query = query
			break
		} else {
			payload, err := parseData(string(resp.GetBody()))
			if err != nil {
				uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
				return
			}
			if len(payload) > 0 {
				response.Query = query
				response.Entries = append(
					response.Entries,
					Entry{STI: nil, Payload: payload},
				)
				break
			}
		}
	}

	uniresp.WriteJSONResponse(ctx.Writer, response)
}

func (aa *SSJCActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.globalCtx.Cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.globalCtx.Cache.Set(req, resp, nil)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &services.SimpleResponse{Err: err}
	}
	return resp
}

func NewSSJCActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *SSJCActions {
	return &SSJCActions{
		globalCtx:       globalCtx,
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
