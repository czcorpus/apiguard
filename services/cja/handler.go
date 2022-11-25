// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cja

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

type NeomatActions struct {
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Content string `json:"content"`
	Image   string `json:"image"`
	CSS     string `json:"css"`
}

func (aa *NeomatActions) Query(w http.ResponseWriter, req *http.Request) {
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
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
	)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	response, err := parseData(responseHTML, aa.conf.BaseURL)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	services.WriteJSONResponse(w, response)
}

func (aa *NeomatActions) createMainRequest(url1 string, url2 string) (string, error) {
	cachedResult, err := aa.cache.Get(url1)
	if err == reqcache.ErrCacheMiss {
		sbody, status, err := services.GetRequest(url1, aa.conf.ClientUserAgent)
		if err != nil {
			return "", err
		}
		if status == 500 {
			sbody, _, err = services.GetRequest(url2, aa.conf.ClientUserAgent)
			if err != nil {
				return "", err
			}
		}
		err = aa.cache.Set(url1, sbody)
		if err != nil {
			return "", err
		}
		return sbody, nil

	} else if err != nil {
		return "", err
	}
	return cachedResult, nil
}

func NewCJAActions(
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
