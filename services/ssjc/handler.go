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
	"fmt"
	"net/http"
	"net/url"
)

type SSJCActions struct {
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
		responseHTML, err := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledat&heslo=%s&where=hesla&hsubstr=no", aa.conf.BaseURL, url.QueryEscape(query)),
			req,
		)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}

		// check if there are multiple results
		STIs, err := lookForSTI(responseHTML)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}

		if len(STIs) > 0 {
			for _, STI := range STIs {
				subResponseHTML, err := aa.createMainRequest(
					fmt.Sprintf("%s/search.php?hledej=Hledat&sti=%d&where=hesla&hsubstr=no", aa.conf.BaseURL, STI),
					req,
				)
				if err != nil {
					services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
					return
				}
				payload, err := parseData(subResponseHTML)
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
			payload, err := parseData(responseHTML)
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

func (aa *SSJCActions) createMainRequest(url string, req *http.Request) (string, error) {
	cachedResult, _, err := aa.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		sbody, _, err := services.GetRequest(url, aa.conf.ClientUserAgent)
		if err != nil {
			return "", err
		}
		err = aa.cache.Set(url, sbody, req)
		if err != nil {
			return "", err
		}
		return sbody, nil

	} else if err != nil {
		return "", err
	}
	return cachedResult, nil
}

func NewSSJCActions(
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *SSJCActions {
	return &SSJCActions{
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
