// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

/*
note:
curl
	--header "Origin: https://slovnikcestiny.cz"
	--header "Referer: https://slovnikcestiny.cz/heslo/batalion/0/548"
	--header "X-Requested-With: XMLHttpRequest" --header "Host: slovnikcestiny.cz"
	https://slovnikcestiny.cz/web_ver_ajax.php
*/

const (
	ServiceName = "assc"
)

type ASSCActions struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

func (aa *ASSCActions) Query(w http.ResponseWriter, req *http.Request) {
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		services.LogEvent(ServiceName, t0, nil, "processed request to 'assc'")
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

	var data *dataStruct
	for _, query := range queries {
		responseHTML, err := aa.createMainRequest(
			fmt.Sprintf("%s/heslo/%s/", aa.conf.BaseURL, url.QueryEscape(query)),
			req,
		)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}
		data, err = parseData(responseHTML)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}
		// check if result is not empty and contains query key
		if data.lastItem != nil {
			data.Query = query
			break
		}
	}
	services.WriteJSONResponse(w, data)
}

func (aa *ASSCActions) createMainRequest(url string, req *http.Request) (string, error) {
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

func NewASSCActions(
	globalCtx *services.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *ASSCActions {
	return &ASSCActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
