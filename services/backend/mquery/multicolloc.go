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

func (collargs *collocArgs) toURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", collargs.q)
	if collargs.subcorpus != "" {
		q.Add("subcorpus", collargs.subcorpus)
	}
	if collargs.matchCase == 1 {
		q.Add("matchCase", strconv.Itoa(collargs.matchCase))
	}
	if collargs.maxItems > 0 {
		q.Add("maxItems", strconv.Itoa(collargs.maxItems))
	}
	if collargs.minItems > 0 {
		q.Add("minItems", strconv.Itoa(collargs.minItems))
	}
	if collargs.minCollFreq > 0 {
		q.Add("minCollFreq", strconv.Itoa(collargs.minCollFreq))
	}
	if collargs.srchAttr != "" {
		q.Add("srchAttr", collargs.srchAttr)
	}
	return q.Encode()
}

// ---------------------------------

type concArgs struct {
	q         string
	subcorpus string
	maxRows   int
}

func (concargs *concArgs) toURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", concargs.q)
	if concargs.subcorpus != "" {
		q.Add("subcorpus", concargs.subcorpus)
	}
	if concargs.maxRows > 0 {
		q.Add("maxRows", strconv.Itoa(concargs.maxRows))
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

func (mp *MQueryProxy) createConcURL(corpusID string, args concArgs) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "concordance", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = args.toURLQuery()
	return url2, nil
}

// ------------------------------------

type MultiCollocSourceArgs struct {
	Action      string `json:"action"`
	CorpusID    string `json:"corpusId"`
	Subcorpus   string `json:"subcorpus"`
	MinFreq     int    `json:"minFreq"`     // minimum frequency of collocation
	MinCorpFreq int    `json:"minCorpFreq"` // minimum frequency of word in corpus
	MinItems    int    `json:"minItems"`    // minimum required number of collocations
	MaxItems    int    `json:"maxItems"`    // max number of returned entries (can be lower than minItems)
}

func (mp *MQueryProxy) tryCollSource(ctx *gin.Context, reqProps guard.ReqEvaluation, q string, arg MultiCollocSourceArgs) (found bool, statusCode int, err error) {
	cArgs := collocArgs{
		subcorpus:   arg.Subcorpus,
		q:           q,
		srchAttr:    "lemma",
		matchCase:   0,
		maxItems:    arg.MaxItems,
		minItems:    arg.MinItems,
		minCollFreq: arg.MinFreq,
	}

	collocExtURL, err := mp.createCollocExtURL(arg.CorpusID, cArgs)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	req := *ctx.Request
	req.URL = collocExtURL
	req.Method = http.MethodGet
	// this is necessary, if making multiple requests with one context
	// otherwise body reader is closed and subsequent requests fail
	if req.Body != nil {
		req.Body = nil
		req.ContentLength = 0
	}

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
	defer ctx.Writer.Flush()

	buffer := make([]byte, 4096)
	hasData := false
	toBeFlushed := false
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			toBeFlushed = true
			hasData = true
			if _, err := ctx.Writer.Write(buffer[:n]); err != nil {
				return hasData, http.StatusInternalServerError, err
			}
		} else if toBeFlushed {
			ctx.Writer.Flush()
			toBeFlushed = false
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

func (mp *MQueryProxy) tryConcSource(ctx *gin.Context, reqProps guard.ReqEvaluation, q string, arg MultiCollocSourceArgs) (found bool, statusCode int, err error) {
	cArgs := concArgs{
		subcorpus: arg.Subcorpus,
		q:         q,
		maxRows:   arg.MaxItems,
	}

	concURL, err := mp.createConcURL(arg.CorpusID, cArgs)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	req := *ctx.Request
	req.URL = concURL
	req.Method = http.MethodGet
	// this is necessary, if making multiple requests with one context
	// otherwise body reader is closed and subsequent requests fail
	if req.Body != nil {
		req.Body = nil
		req.ContentLength = 0
	}

	resp := mp.HandleRequest(&req, reqProps, false)
	statusCode = resp.Response().GetStatusCode()
	if err := resp.Error(); err != nil {
		return false, statusCode, err
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		return false, statusCode, err
	}
	fmt.Fprintf(ctx.Writer, "data: %s", respBody)
	return true, statusCode, nil
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

	query := ctx.Query("q")
	if query == "" {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("missing required query parameter: q"), http.StatusBadRequest)
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
		log.Info().Msgf("Processing source %d/%d: %+v", i+1, len(args), arg)
		var hasData bool
		var sc int
		var err error

		switch arg.Action {
		case "coll":
			hasData, sc, err = mp.tryCollSource(ctx, reqProps, query, arg)
		case "conc":
			hasData, sc, err = mp.tryConcSource(ctx, reqProps, query, arg)
		default:
			continue
		}

		statusCode = sc
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("failed to request data from source: %w", err), statusCode)
			return
		} else if hasData {
			log.Info().Msgf("Successfully streamed data from source %d/%d", i+1, len(args))
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
