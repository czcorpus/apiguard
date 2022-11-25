// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neomat

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"wum/botwatch"
	"wum/reqcache"
	"wum/services"
)

/*
note:
curl
	--header "Origin: https://slovnikcestiny.cz"
	--header "Referer: https://slovnikcestiny.cz/heslo/batalion/0/548"
	--header "X-Requested-With: XMLHttpRequest" --header "Host: slovnikcestiny.cz"
	https://slovnikcestiny.cz/web_ver_ajax.php
*/

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
		fmt.Sprintf("%s/index.php?retezec=%s&prijimam=1", aa.conf.BaseURL, url.QueryEscape(query)))
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

func (aa *NeomatActions) createMainRequest(url string) (string, error) {
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
