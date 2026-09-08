// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2026 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mquery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ---------------------------------

type collocArgs struct {
	q           string
	subcorpus   string
	srchAttr    string
	matchCase   int
	maxItems    int
	minItems    int
	minCollFreq int
}

func (lfargs *collocArgs) toURLQuery() string {
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
	if lfargs.minItems > 0 {
		q.Add("minItems", strconv.Itoa(lfargs.minItems))
	}
	if lfargs.minCollFreq > 0 {
		q.Add("minCollFreq", strconv.Itoa(lfargs.minCollFreq))
	}
	if lfargs.srchAttr != "" {
		q.Add("srchAttr", lfargs.srchAttr)
	}
	return q.Encode()
}

// ---------------------------------

func (mp *MQueryProxy) createCollocExtURL(corpusID string, args collocArgs) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "collocations-extended", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed collocation URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed collocation URL: %w", err)
	}
	url2.RawQuery = args.toURLQuery()
	return url2, nil
}

// ------------------------------------

type MultiCollocSourceArgs struct {
	CorpusID    string `json:"corpusId"`
	MinFreq     int    `json:"minFreq"`     // minimum frequency of collocation
	MinCorpFreq int    `json:"minCorpFreq"` // minimum frequency of word in corpus
	MinItems    int    `json:"minItems"`    // minimum required number of collocations
}

func (mp *MQueryProxy) tryCollSource(ctx *gin.Context, reqProps guard.ReqEvaluation, q string, maxItems int, arg MultiCollocSourceArgs) (found bool, statusCode int, err error) {
	cArgs := collocArgs{
		q:           q,
		srchAttr:    "lemma",
		matchCase:   0,
		maxItems:    maxItems,
		minItems:    arg.MinItems,
		minCollFreq: arg.MinFreq,
	}

	collocExtURL, err := mp.createCollocExtURL(arg.CorpusID, cArgs)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	req := *ctx.Request
	req.URL = collocExtURL
	req.Method = "GET"
	// this is necessary otherwise first request closes reader
	// and subsequent requests fail
	req.Body = http.NoBody
	req.ContentLength = 0

	resp := mp.MakeStreamRequest(&req, reqProps)
	backend := resp.Response()

	statusCode = backend.GetStatusCode()
	if statusCode >= 400 {
		if err := resp.Error(); err != nil {
			return false, statusCode, err
		}
		return false, statusCode, nil
	}
	reader := backend.GetBodyReader()
	if reader == nil {
		return false, statusCode, nil
	}
	defer backend.CloseBodyReader()

	buffer := make([]byte, 4096)
	hasData := false
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			hasData = true
			if _, err := ctx.Writer.Write(buffer[:n]); err != nil {
				return hasData, http.StatusInternalServerError, err
			}
			ctx.Writer.Flush()
		}
		if err != nil {
			if err != io.EOF {
				return hasData, http.StatusInternalServerError, err
			}
			break
		}
	}
	return hasData, statusCode, nil
}

func (mp *MQueryProxy) MultiCollocExtended(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, firstPartyAPICall bool
	var statusCode int
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	defer mp.LogRequest(ctx, &humanID, &firstPartyAPICall, &cached, t0)

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
		ctx, humanID, userID, &firstPartyAPICall,
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

	Q := ctx.Query("q")
	if Q == "" {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("missing required query parameter: q"), http.StatusBadRequest)
		return
	}
	maxItems, err := strconv.Atoi(ctx.DefaultQuery("maxItems", "10"))
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("invalid maxItems parameter: %w", err), http.StatusBadRequest)
		return
	}

	var args []MultiCollocSourceArgs
	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to parse request body: %w", err), http.StatusBadRequest)
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	for i, arg := range args {
		log.Info().Msgf("Processing collocation source %d/%d: %+v", i+1, len(args), arg)
		hasData, sc, err := mp.tryOneSource(ctx, reqProps, Q, maxItems, arg)
		statusCode = sc
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("failed to request data from collocation source: %w", err), statusCode)
			return
		} else if hasData {
			log.Info().Msgf("Successfully streamed collocation data from source %d/%d", i+1, len(args))
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
