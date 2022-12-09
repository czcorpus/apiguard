// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ssjc

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/logging"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	ServiceName = "ssjc"
)

type SSJCActions struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
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

func (aa *SSJCActions) Query(w http.ResponseWriter, req *http.Request) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		logging.LogServiceRequest(ServiceName, t0, &cached, nil)
	}()

	queries, ok := req.URL.Query()["q"]
	if !ok {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty query"), 422)
		return
	}
	if len(queries) != 1 && len(queries) > aa.conf.MaxQueries {
		services.WriteJSONErrorResponse(w, services.NewActionError("too many queries"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}

	response := Response{Entries: make([]Entry, 0)}
	var query string
	for _, query = range queries {
		resp := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledat&heslo=%s&where=hesla&hsubstr=no", aa.conf.BaseURL, url.QueryEscape(query)),
			req,
		)
		cached = cached || resp.IsCached()
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}

		// check if there are multiple results
		STIs, err := lookForSTI(string(resp.GetBody()))
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}

		if len(STIs) > 0 {
			for _, STI := range STIs {
				resp := aa.createMainRequest(
					fmt.Sprintf("%s/search.php?hledej=Hledat&sti=%d&where=hesla&hsubstr=no", aa.conf.BaseURL, STI),
					req,
				)
				cached = cached || resp.IsCached()
				if err != nil {
					services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
					return
				}
				payload, err := parseData(string(resp.GetBody()))
				if err != nil {
					services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
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
				services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
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

	services.WriteJSONResponse(w, response)
}

func (aa *SSJCActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := aa.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		resp = services.GetRequest(url, aa.conf.ClientUserAgent)
		err = aa.cache.Set(req, resp)
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
	globalCtx *services.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *SSJCActions {
	return &SSJCActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
