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
}

func (aa *SSJCActions) Query(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	if query == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty query"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}
	responseHTML, err := aa.createMainRequest(
		fmt.Sprintf("%s/search.php?hledej=Hledat&heslo=%s&where=hesla&hsubstr=no", aa.conf.BaseURL, url.QueryEscape(query)))
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

	response := Response{Entries: make([]Entry, 0)}
	if len(STIs) > 0 {
		for _, STI := range STIs {
			subResponseHTML, err := aa.createMainRequest(
				fmt.Sprintf("%s/search.php?hledej=Hledat&sti=%d&where=hesla&hsubstr=no", aa.conf.BaseURL, STI))
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
	} else {
		payload, err := parseData(responseHTML)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}
		if len(payload) > 0 {
			response.Entries = append(
				response.Entries,
				Entry{STI: nil, Payload: payload},
			)
		}
	}

	services.WriteJSONResponse(w, response)
}

func (aa *SSJCActions) createMainRequest(url string) (string, error) {
	cachedResult, err := aa.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		sbody, _, err := services.GetRequest(url, aa.conf.ClientUserAgent)
		if err != nil {
			return "", err
		}
		err = aa.cache.Set(url, sbody)
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
