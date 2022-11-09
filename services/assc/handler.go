// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

import (
	"fmt"
	"net/http"
	"net/url"
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

type ASSCActions struct {
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

func (aa *ASSCActions) Query(w http.ResponseWriter, req *http.Request) {
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
		fmt.Sprintf("%s/heslo/%s/", aa.conf.BaseURL, url.QueryEscape(query)))
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	data, err := parseData(responseHTML)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	services.WriteJSONResponse(w, data)
}

func (aa *ASSCActions) createMainRequest(url string) (string, error) {
	cachedResult, err := aa.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		sbody, err := services.GetRequest(url, aa.conf.ClientUserAgent)
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

func NewASSCActions(conf *Conf, cache services.Cache, analyzer *botwatch.Analyzer) *ASSCActions {
	return &ASSCActions{
		conf:     conf,
		cache:    cache,
		analyzer: analyzer,
	}
}
