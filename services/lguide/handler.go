// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"
	"wum/botwatch"
	"wum/config"
	"wum/logging"
	"wum/services"
	"wum/storage"
)

const (
	targetServiceURLPath     = "/?slovo=%s"
	targetServicePingURLPath = "?id=%s&action=single"
)

type LanguageGuideActions struct {
	conf     config.LanguageGuideConf
	watchdog *botwatch.Watchdog[*logging.LGRequestRecord]
	analyzer *botwatch.Analyzer
}

func (lga *LanguageGuideActions) createRequest(url string) (string, error) {
	client := &http.Client{}
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
	query := req.URL.Query().Get("q")
	if query == "" {
		services.WriteJSONErrorResponse(w, services.NewActionError("Empty query"), 422)
		return
	}

	// handle bots
	isLegit, err := lga.analyzer.Analyze(req)
	if err != nil {
		log.Print("ERROR: failed to analyze client ", err)
	}
	if !isLegit {
		// TODO wait some more time
		log.Print("Suspicious client detected - let's wait a few seconds")
		time.Sleep(time.Duration(5) * time.Second)
	}

	resp, err := http.Get(fmt.Sprintf(lga.conf.BaseURL+targetServiceURLPath, url.QueryEscape(query)))

	lga.watchdog.Add(logging.NewLGRequestRecord(req))

	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	defer resp.Body.Close()
	responseSrc, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	parsed := Parse(string(responseSrc))
	lga.triggerDummyRequests(query, parsed)
	services.WriteJSONResponse(w, parsed)
}

func NewLanguageGuideActions(
	conf config.LanguageGuideConf,
	botConf botwatch.BotDetectionConf,
	db *storage.MySQLAdapter,
	analyzer *botwatch.Analyzer,
) *LanguageGuideActions {
	wdog := botwatch.NewLGWatchdog(botConf, db)
	return &LanguageGuideActions{
		conf:     conf,
		watchdog: wdog,
		analyzer: analyzer,
	}
}
