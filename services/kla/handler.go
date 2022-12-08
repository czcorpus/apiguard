// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kla

import (
	"apiguard/botwatch"
	"apiguard/reqcache"
	"apiguard/services"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	ServiceName = "kla"
)

type KLAActions struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        *botwatch.Analyzer
}

type Response struct {
	Images []string `json:"images"`
	Query  string   `json:"query"`
}

func (aa *KLAActions) Query(w http.ResponseWriter, req *http.Request) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		services.LogServiceRequest(ServiceName, t0, &cached, nil)
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
	maxImages := req.URL.Query().Get("maxImages")
	if maxImages == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty maxImages"), 422)
		return
	}
	maxImageCount, err := strconv.Atoi(maxImages)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}

	err = services.RestrictResponseTime(w, req, aa.readTimeoutSecs, aa.analyzer)
	if err != nil {
		return
	}

	var images []string
	var query string
	for _, query = range queries {
		responseHTML, cached2, err := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledej&heslo=%s&where=hesla&zobraz_cards=cards&pocet_karet=100&not_initial=1", aa.conf.BaseURL, url.QueryEscape(query)),
			req,
		)
		cached = cached || cached2
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}

		images, err = parseData(responseHTML, maxImageCount, aa.conf.BaseURL)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}
		if len(images) > 0 {
			break
		}
	}

	services.WriteJSONResponse(w, Response{
		Images: images,
		Query:  query,
	})
}

func (aa *KLAActions) createMainRequest(url string, req *http.Request) (string, bool, error) {
	cachedResult, _, err := aa.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		sbody, _, err := services.GetRequest(url, aa.conf.ClientUserAgent)
		if err != nil {
			return "", false, err
		}
		err = aa.cache.Set(req, sbody, nil)
		if err != nil {
			return "", false, err
		}
		return sbody, false, nil

	} else if err != nil {
		return "", false, err
	}
	return cachedResult, true, nil
}

func NewKLAActions(
	globalCtx *services.GlobalContext,
	conf *Conf,
	cache services.Cache,
	analyzer *botwatch.Analyzer,
	readTimeoutSecs int,
) *KLAActions {
	return &KLAActions{
		globalCtx:       globalCtx,
		conf:            conf,
		cache:           cache,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
