// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cja

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
)

type NeomatActions struct {
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Content  string `json:"content"`
	Image    string `json:"image"`
	CSS      string `json:"css"`
	Backlink string `json:"backlink"`
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
	responseHTML, backlink, err := aa.createRequests(
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		req,
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
	response.Backlink = backlink

	services.WriteJSONResponse(w, response)
}

func (aa *NeomatActions) createSubRequest(url string, req *http.Request) (string, int, error) {
	cachedResult, _, err := aa.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		sbody, status, err := services.GetRequest(url, aa.conf.ClientUserAgent)
		if err != nil {
			return "", 0, err
		}
		err = aa.cache.Set(url, sbody, nil, req)
		if err != nil {
			return "", 0, err
		}
		return sbody, status, nil
	}
	return cachedResult, 200, nil
}

func (aa *NeomatActions) createRequests(url1 string, url2 string, req *http.Request) (string, string, error) {
	result, status, err := aa.createSubRequest(url1, req)
	if err != nil {
		return "", "", err
	}
	if status == 500 {
		result, _, err = aa.createSubRequest(url2, req)
		if err != nil {
			return "", "", err
		}
		return result, url2, nil
	}
	return result, url1, nil
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
