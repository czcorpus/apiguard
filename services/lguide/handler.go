// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"
	"wum/botwatch"
	"wum/logging"
	"wum/reqcache"
	"wum/services"
	"wum/storage"
	"wum/telemetry"

	"github.com/rs/zerolog/log"
)

const (
	targetServiceURLPath       = "/?slovo=%s"
	targetDirectServiceURLPath = "/?id=%s"
	targetServicePingURLPath   = "?id=%s&action=single"
)

type LanguageGuideActions struct {
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

func (lga *LanguageGuideActions) createMainRequest(url string) (string, error) {
	cachedResult, err := lga.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		sbody, err := services.GetRequest(url, lga.conf.ClientUserAgent)
		err = lga.cache.Set(url, sbody)
		if err != nil {
			return "", err
		}
		return sbody, nil

	} else if err != nil {
		return "", err
	}
	return cachedResult, nil
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
	lga.watchdog.Add(logging.NewLGRequestRecord(req))

	query := req.URL.Query().Get("q")
	if query == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("empty query"), 422)
		return
	}

	err := services.RestrictResponseTime(w, req, lga.readTimeoutSecs, lga.analyzer)
	if err != nil {
		return
	}

	var responseHTML string
	direct := req.URL.Query().Get("direct")
	if direct == "1" {
		responseHTML, err = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(query)))
	} else {
		responseHTML, err = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetServiceURLPath, url.QueryEscape(query)))
	}

	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	parsed := Parse(responseHTML)
	if len(parsed.Alternatives) > 0 {
		alts := parsed.Alternatives
		responseHTML, err = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(alts[0].Id)))

		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
			return
		}
		parsed = Parse(responseHTML)
		parsed.Alternatives = alts
	}

	if len(parsed.items) > 0 {
		log.Info().Msgf("More data available for `%s` in `items`: %v", query, parsed.items)
	}
	if parsed.Error != nil {
		services.WriteJSONErrorResponse(w, services.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	lga.triggerDummyRequests(query, parsed)
	services.WriteJSONResponse(w, parsed)
}

func NewLanguageGuideActions(
	conf *Conf,
	botwatchConf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
	readTimeoutSecs int,
	db *storage.MySQLAdapter,
	analyzer *botwatch.Analyzer,
	cache services.Cache,
) *LanguageGuideActions {
	wdog := botwatch.NewLGWatchdog(botwatchConf, telemetryConf, db)
	return &LanguageGuideActions{
		conf:            conf,
		readTimeoutSecs: readTimeoutSecs,
		watchdog:        wdog,
		analyzer:        analyzer,
		cache:           cache,
	}
}
