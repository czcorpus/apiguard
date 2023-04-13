// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/ctx"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/logging"
	"apiguard/services/telemetry"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

const (
	ServiceName                = "lguide"
	targetServiceURLPath       = "/?slovo=%s"
	targetDirectServiceURLPath = "/?id=%s"
	targetServicePingURLPath   = "?id=%s&action=single"
)

type LanguageGuideActions struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	watchdog        *botwatch.Watchdog[*logging.LGRequestRecord]
	analyzer        *botwatch.Analyzer
	cache           services.Cache
}

func (lga *LanguageGuideActions) createRequest(url string) (string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", lga.conf.ClientUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (lga *LanguageGuideActions) createMainRequest(url string, req *http.Request) services.BackendResponse {
	resp, err := lga.cache.Get(req, nil)
	if err == reqcache.ErrCacheMiss {
		resp := services.GetRequest(url, lga.conf.ClientUserAgent)
		err = lga.cache.Set(req, resp, nil)
		if err != nil {
			return &services.SimpleResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &services.SimpleResponse{Err: err}
	}
	return resp
}

func (lga *LanguageGuideActions) createResourceRequest(url string) error {
	time.Sleep(500 * time.Millisecond)
	_, err := lga.createRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (lga *LanguageGuideActions) triggerDummyRequests(query string, data *ParsedData) {
	urls := make([]string, 0, len(data.CSSLinks)+len(data.Scripts)+1)
	urls = append(urls, data.Scripts...)
	urls = append(urls, data.CSSLinks...)
	for i, u := range urls {
		urls[i] = lga.conf.BaseURL + u
	}
	urls = append(urls, fmt.Sprintf(lga.conf.BaseURL+targetServicePingURLPath, query))
	rand.Shuffle(len(urls), func(i, j int) {
		urls[i], urls[j] = urls[j], urls[i]
	})
	for _, url := range urls {
		go func(curl string) {
			time.Sleep(time.Duration(time.Duration(math.RoundToEven(rand.NormFloat64()+1.5*1000)) * time.Millisecond))
			lga.createResourceRequest(curl)
		}(url)
	}
}

func (lga *LanguageGuideActions) Query(w http.ResponseWriter, req *http.Request) {
	var cached bool
	t0 := time.Now().In(lga.globalCtx.TimezoneLocation)
	defer func() {
		lga.globalCtx.BackendLogger.Log(ServiceName, time.Since(t0), &cached, nil)
	}()

	lga.watchdog.Add(logging.NewLGRequestRecord(req))

	query := req.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError("empty query"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, lga.readTimeoutSecs, lga.analyzer)
	if err != nil {
		return
	}

	var resp services.BackendResponse
	direct := req.URL.Query().Get("direct")
	if direct == "1" {
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(query)),
			req,
		)
	} else {
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetServiceURLPath, url.QueryEscape(query)),
			req,
		)
	}

	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
		return
	}
	parsed := Parse(string(resp.GetBody()))
	if len(parsed.Alternatives) > 0 {
		alts := parsed.Alternatives
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(alts[0].Id)),
			req,
		)

		if err != nil {
			uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
			return
		}
		parsed = Parse(string(resp.GetBody()))
		parsed.Alternatives = alts
	}

	if len(parsed.items) > 0 {
		log.Info().Msgf("More data available for `%s` in `items`: %v", query, parsed.items)
	}
	if parsed.Error != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	lga.triggerDummyRequests(query, parsed)
	uniresp.WriteJSONResponse(w, parsed)
}

func NewLanguageGuideActions(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	botwatchConf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
	readTimeoutSecs int,
	db *cncdb.DelayStats,
	analyzer *botwatch.Analyzer,
	cache services.Cache,
) *LanguageGuideActions {
	wdog := botwatch.NewLGWatchdog(botwatchConf, telemetryConf, db)
	return &LanguageGuideActions{
		globalCtx:       globalCtx,
		conf:            conf,
		readTimeoutSecs: readTimeoutSecs,
		watchdog:        wdog,
		analyzer:        analyzer,
		cache:           cache,
	}
}
