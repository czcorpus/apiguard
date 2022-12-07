// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neomat

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type NeomatActions struct {
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Entries []string `json:"entries"`
}

func (aa *NeomatActions) Query(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	if query == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty query"), 422)
		return
	}
	maxItems := req.URL.Query().Get("maxItems")
	if maxItems == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty maxItems"), 422)
		return
	}
	maxItemsCount, err := strconv.Atoi(maxItems)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	err = services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}
	responseHTML, err := aa.createMainRequest(
		fmt.Sprintf("%s/index.php?retezec=%s&prijimam=1", aa.conf.BaseURL, url.QueryEscape(query)),
		req,
	)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	entries, err := parseData(responseHTML, maxItemsCount)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	services.WriteJSONResponse(w, Response{Entries: entries})
}

func (aa *NeomatActions) createMainRequest(url string, req *http.Request) (string, error) {
	cachedResult, _, err := aa.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		sbody, _, err := services.GetRequest(url, aa.conf.ClientUserAgent)
		if err != nil {
			return "", err
		}
		err = aa.cache.Set(req, sbody, nil)
		if err != nil {
			return "", err
		}
		return sbody, nil

	} else if err != nil {
		return "", err
	}
	return cachedResult, nil
}

func NewNeomatActions(
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *NeomatActions {
	return &NeomatActions{
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
