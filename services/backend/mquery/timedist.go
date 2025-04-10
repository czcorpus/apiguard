// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/reporting"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// streamedFreqDistArgs is basically a copy of MQuery's unexported streamedFreqsBaseArgs
type streamedFreqDistArgs struct {
	q        string
	attr     string
	fcrit    string
	flimit   int
	maxItems int
	event    string
}

func (sfargs *streamedFreqDistArgs) toURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", sfargs.q)
	q.Add("attr", sfargs.attr)
	if sfargs.flimit > 1 {
		q.Add("flimit", strconv.Itoa(sfargs.flimit))
	}
	if sfargs.maxItems > 0 {
		q.Add("maxItems", strconv.Itoa(sfargs.maxItems))
	}
	if sfargs.maxItems > 0 {
		q.Add("maxItems", strconv.Itoa(sfargs.maxItems))
	}
	if sfargs.event != "" {
		q.Add("event", sfargs.event)
	}
	return q.Encode()
}

// ---------------------------------

type lemmaFreqResponse struct {
	Freqs FreqDistribItemList `json:"freqs"`

	Error error `json:"error,omitempty"`
}

// ---------------------------------

type lemmaFreqDistArgs struct {
	q         string
	subcorpus string
	attr      string
	fcrit     string
	matchCase int
	maxItems  int
	flimit    int
}

func (lfargs *lemmaFreqDistArgs) toURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", lfargs.q)
	if lfargs.subcorpus != "" {
		q.Add("subcorpus", lfargs.subcorpus)
	}
	if lfargs.matchCase == 1 {
		q.Add("matchCase", strconv.Itoa(lfargs.matchCase))
	}
	if lfargs.maxItems > 0 {
		q.Add("maxItems", strconv.Itoa(lfargs.maxItems))
	}
	if lfargs.flimit > 0 {
		q.Add("flimit", strconv.Itoa(lfargs.flimit))
	}
	if lfargs.fcrit != "" {
		q.Add("fcrit", lfargs.fcrit)

	} else if lfargs.attr != "" {
		q.Add("attr", lfargs.attr)
	}
	return q.Encode()
}

// ---------------------------------

func (mp *MQueryProxy) createLemmaFreqsURL(corpusID string, args lemmaFreqDistArgs) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.CoreProxy.BackendURL.String(), mp.EnvironConf().ServicePath, "freqs", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed time dist. URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = args.toURLQuery()
	return url2, nil
}

func (mp *MQueryProxy) createTimeDistURL(corpusID string, args streamedFreqDistArgs) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.CoreProxy.BackendURL.String(), mp.EnvironConf().ServicePath, "freqs-by-year-streamed", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed time dist. URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = args.toURLQuery()
	return url2, nil
}

// ------------------------------------

func (mp *MQueryProxy) TimeDistAltWord(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	var statusCode int
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	defer mp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	// guard request

	if !strings.HasPrefix(ctx.Request.URL.Path, mp.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to get speeches - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := mp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err := mp.Guard().DetermineTrueUserID(ctx.Request)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract human user ID information")
		http.Error(ctx.Writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if humanID == common.InvalidUserID {
		humanID = reqProps.ClientID
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: humanID,
	}

	if err := guard.RestrictResponseTime(
		ctx.Writer, ctx.Request, mp.EnvironConf().ReadTimeoutSecs, mp.Guard(), clientID,
	); err != nil {
		return
	}

	if err := mp.ProcessReqHeaders(
		ctx, humanID, userID, &indirectAPICall,
	); err != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to get speeches - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// process request

	rt0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	corpusID := ctx.Query("corpname")
	// first, load freq dist. by lemma and select the most freq. one
	lmArgs := lemmaFreqDistArgs{
		q:         ctx.Query("q"),
		attr:      "lemma",
		matchCase: 0,
		maxItems:  10,
		flimit:    5,
	}
	lemmaFreqURL, err := mp.createLemmaFreqsURL(corpusID, lmArgs)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to process: %w", err), http.StatusBadRequest)
		return
	}
	req1 := *ctx.Request
	req1.URL = lemmaFreqURL
	req1.Method = "GET"
	serviceResp := mp.MakeRequest(&req1, reqProps)

	var lemmaData lemmaFreqResponse
	defer serviceResp.GetBodyReader().Close()
	resp1Body, err := io.ReadAll(serviceResp.GetBodyReader())
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to process: %w", err), http.StatusInternalServerError)
		return
	}
	if err := sonic.Unmarshal(resp1Body, &lemmaData); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to process: %w", err), http.StatusInternalServerError)
		return
	}
	spew.Dump(lemmaData)

	// then call "classic" streamed time dist

	q := util.Ternary(
		len(lemmaData.Freqs) > 0,
		fmt.Sprintf(`[lemma="%s"]`, lemmaData.Freqs[0].Word),
		ctx.Query("q"),
	)

	flimit, err := strconv.Atoi(ctx.DefaultQuery("flimit", "10"))
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("flimit arg: %w", err), http.StatusBadRequest)
		return
	}
	url2, err := mp.createTimeDistURL(
		corpusID,
		streamedFreqDistArgs{
			q:        q,
			attr:     ctx.Query("attr"),
			flimit:   flimit,
			maxItems: 100, // TODO
			event:    ctx.Query("event"),
		},
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to generate time dist url: %w", err), http.StatusBadRequest)
		return
	}
	req2 := *ctx.Request
	req2.URL = url2
	req2.Method = "GET"
	resp2 := mp.MakeStreamRequest(&req2, reqProps)
	statusCode = resp2.GetStatusCode()

	ctx.Writer.Header().Set("Content-Type", resp2.GetHeaders().Get("Content-Type"))
	ctx.Writer.WriteHeader(resp2.GetStatusCode())

	buffer := make([]byte, 4096)
	for {
		n, err := resp2.GetBodyReader().Read(buffer)
		if n > 0 {
			_, writeErr := ctx.Writer.Write(buffer[:n])
			if writeErr != nil {
				return
			}
		}
		if err != nil {
			// for any kind of error (incl. io.EOF),
			// there is nothing we can do here
			break
		}
	}

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}
