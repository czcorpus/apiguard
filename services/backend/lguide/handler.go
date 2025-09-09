// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"apiguard/botwatch"
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/tlmtr"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/logging"
	"apiguard/telemetry"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	ServiceName                = "lguide"
	targetServiceURLPath       = "/?slovo=%s"
	targetDirectServiceURLPath = "/?id=%s"
	targetServicePingURLPath   = "?id=%s&action=single"
)

type LanguageGuideActions struct {
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	watchdog        *Watchdog[*logging.LGRequestRecord]
	guard           guard.ServiceGuard
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (lga *LanguageGuideActions) createMainRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, lga.conf.ClientUserAgent, lga.globalCtx.Cache)
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

func (lga *LanguageGuideActions) Query(ctx *gin.Context) {
	var cached bool
	t0 := time.Now().In(lga.globalCtx.TimezoneLocation)
	defer func() {
		lga.globalCtx.BackendLoggers.Get(lga.serviceKey).Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			common.InvalidUserID,
			false,
			reporting.BackendActionTypeQuery,
		)
	}()

	lga.watchdog.Add(logging.NewLGRequestRecord(ctx.Request))

	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, lga.readTimeoutSecs, lga.guard, clientID)
	if err != nil {
		return
	}

	var resp proxy.ResponseProcessor
	direct := ctx.Request.URL.Query().Get("direct")
	if direct == "1" {
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(query)),
			ctx.Request,
		)
	} else {
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetServiceURLPath, url.QueryEscape(query)),
			ctx.Request,
		)
	}

	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}
	parsed := Parse(string(respBody))
	if len(parsed.Alternatives) > 0 {
		alts := parsed.Alternatives
		resp = lga.createMainRequest(
			fmt.Sprintf(lga.conf.BaseURL+targetDirectServiceURLPath, url.QueryEscape(alts[0].Id)),
			ctx.Request,
		)

		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		parsed = Parse(string(respBody))
		parsed.Alternatives = alts
	}

	if len(parsed.items) > 0 {
		log.Info().Msgf("More data available for `%s` in `items`: %v", query, parsed.items)
	}
	if parsed.Error != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	lga.triggerDummyRequests(query, parsed)
	uniresp.WriteJSONResponse(ctx.Writer, parsed)
}

func NewLanguageGuideActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	botwatchConf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
	readTimeoutSecs int,
	guard *tlmtr.Guard,
) *LanguageGuideActions {
	wdog := NewLGWatchdog(botwatchConf, telemetryConf, globalCtx.TelemetryDB)
	return &LanguageGuideActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		readTimeoutSecs: readTimeoutSecs,
		watchdog:        wdog,
		guard:           guard,
	}
}
