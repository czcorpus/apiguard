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
	"encoding/json"
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

func newRequest(ctx *gin.Context, method string, url *url.URL) http.Request {
	req := *ctx.Request
	req.URL = url
	req.Method = method
	// this is necessary, if making multiple requests with one context
	// otherwise body reader is closed and subsequent requests fail
	if req.Body != nil {
		req.Body = nil
		req.ContentLength = 0
	}
	return req
}

// ---------------- Args --------------------

type SourceArgs interface {
	ToQuery(q string, event string) string
}

type CollExtArgs struct {
	Q               string `json:"q"`
	CmpCorp         string `json:"cmpCorp"`
	Subcorpus       string `json:"subcorpus"`
	Measure         string `json:"measure"`
	SrchLeft        int    `json:"srchLeft"`
	SrchRight       int    `json:"srchRight"`
	SrchAttr        string `json:"srchAttr"`
	MinCollFreq     int    `json:"minCollFreq"`
	MaxItems        int    `json:"maxItems"`
	ExamplesPerColl int    `json:"examplesPerColl"`
	Event           string `json:"event"`
	MinItems        int    `json:"minItems"`
}

func (a CollExtArgs) ToQuery(q string, event string) string {
	a.Q = q
	a.Event = event

	params := url.Values{}
	if a.Q != "" {
		params.Set("q", a.Q)
	}
	if a.CmpCorp != "" {
		params.Set("cmpCorp", a.CmpCorp)
	}
	if a.Subcorpus != "" {
		params.Set("subcorpus", a.Subcorpus)
	}
	if a.Measure != "" {
		params.Set("measure", a.Measure)
	}
	if a.Measure != "" {
		params.Set("measure", a.Measure)
	}
	params.Set("srchLeft", strconv.Itoa(a.SrchLeft))
	params.Set("srchRight", strconv.Itoa(a.SrchRight))
	if a.SrchAttr != "" {
		params.Set("srchAttr", a.SrchAttr)
	}
	if a.MinCollFreq > 0 {
		params.Set("minCollFreq", strconv.Itoa(a.MinCollFreq))
	}
	if a.MaxItems > 0 {
		params.Set("maxItems", strconv.Itoa(a.MaxItems))
	}
	if a.ExamplesPerColl > 0 {
		params.Set("examplesPerColl", strconv.Itoa(a.ExamplesPerColl))
	}
	if a.MinItems > 0 {
		params.Set("minItems", strconv.Itoa(a.MinItems))
	}
	if a.Event != "" {
		params.Set("event", a.Event)
	}
	return params.Encode()
}

type ConcArgs struct {
	Q             string `json:"q"`
	QueryIdx      int    `json:"queryIdx"`
	MaxRows       int    `json:"maxRows"`
	RowsOffset    int    `json:"rowsOffset"`
	ContextWidth  int    `json:"contextWidth"`
	ContextStruct string `json:"contextStruct"`
	ShowTextProps string `json:"showTextProps"`
}

func (a ConcArgs) ToQuery(q string, event string) string {
	a.Q = q
	params := url.Values{}
	if a.Q != "" {
		params.Set("q", a.Q)
	}
	params.Set("queryIdx", strconv.Itoa(a.QueryIdx))
	if a.MaxRows > 0 {
		params.Set("maxRows", strconv.Itoa(a.MaxRows))
	}
	if a.RowsOffset > 0 {
		params.Set("rowsOffset", strconv.Itoa(a.RowsOffset))
	}
	if a.ContextWidth > 0 {
		params.Set("contextWidth", strconv.Itoa(a.ContextWidth))
	}
	if a.ContextStruct != "" {
		params.Set("contextStruct", a.ContextStruct)
	}
	if a.ShowTextProps != "" {
		params.Set("showTextProps", a.ShowTextProps)
	}
	return params.Encode()
}

type MultiCollocSourceArgs struct {
	Action   string     `json:"action"`
	CorpusID string     `json:"corpusId"`
	Args     SourceArgs `json:"-"`
}

func (mcsa *MultiCollocSourceArgs) UnmarshalJSON(data []byte) error {
	var shim struct {
		Action   string          `json:"action"`
		CorpusID string          `json:"corpusId"`
		Args     json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(data, &shim); err != nil {
		return err
	}
	mcsa.Action = shim.Action
	mcsa.CorpusID = shim.CorpusID

	switch shim.Action {
	case "coll":
		var a CollExtArgs
		if err := json.Unmarshal(shim.Args, &a); err != nil {
			return fmt.Errorf("failed to parse coll args: %w", err)
		}
		mcsa.Args = a
	case "conc":
		var a ConcArgs
		if err := json.Unmarshal(shim.Args, &a); err != nil {
			return fmt.Errorf("failed to parse conc args: %w", err)
		}
		mcsa.Args = a
	default:
		return fmt.Errorf("unknown action: %q", shim.Action)
	}
	return nil
}

// ---------------- Collocation -------------

func (mp *MQueryProxy) createCollocExtURL(args MultiCollocSourceArgs, q string, event string) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "collocations-extended", args.CorpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed collocation URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create streamed collocation URL: %w", err)
	}
	url2.RawQuery = args.Args.ToQuery(q, event)
	return url2, nil
}

func (mp *MQueryProxy) tryCollSource(ctx *gin.Context, reqProps guard.ReqEvaluation, q string, arg MultiCollocSourceArgs, event string) (found bool, statusCode int, err error) {
	collocExtURL, err := mp.createCollocExtURL(arg, q, event)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	req := newRequest(ctx, http.MethodGet, collocExtURL)
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

// ---------------- Concordance -------------

func (mp *MQueryProxy) createConcURL(args MultiCollocSourceArgs, q string) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "concordance", args.CorpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = args.Args.ToQuery(q, "")
	return url2, nil
}

func (mp *MQueryProxy) tryConcSource(ctx *gin.Context, reqProps guard.ReqEvaluation, q string, arg MultiCollocSourceArgs, event string) (found bool, statusCode int, err error) {
	concURL, err := mp.createConcURL(arg, q)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	req := newRequest(ctx, http.MethodGet, concURL)
	resp := mp.HandleRequest(&req, reqProps, false)
	statusCode = resp.Response().GetStatusCode()
	if err := resp.Error(); err != nil {
		return false, statusCode, err
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		return false, statusCode, err
	}
	if event != "" {
		_, err = fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", event, respBody)
	} else {
		_, err = fmt.Fprintf(ctx.Writer, "data: %s\n\n", respBody)
	}
	if err != nil {
		return false, statusCode, err
	}
	return true, statusCode, nil
}

// ---------------- Handler -----------------

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

	event := ctx.Query("event")
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
			hasData, sc, err = mp.tryCollSource(ctx, reqProps, query, arg, event)
		case "conc":
			hasData, sc, err = mp.tryConcSource(ctx, reqProps, query, arg, event)
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
