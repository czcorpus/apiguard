// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
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
	fromYear int
	toYear   int
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
	if sfargs.fcrit != "" {
		q.Add("fcrit", sfargs.fcrit)

	} else if sfargs.attr != "" {
		q.Add("attr", sfargs.attr)
	}
	if sfargs.fromYear != 0 {
		q.Add("fromYear", strconv.Itoa(sfargs.fromYear))
	}
	if sfargs.toYear != 0 {
		q.Add("toYear", strconv.Itoa(sfargs.toYear))
	}
	return q.Encode()
}

// ---------------------------------

type lemmaFreqResponse struct {
	Freqs FreqDistribItemList `json:"freqs"`

	Error string `json:"error,omitempty"`
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
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "freqs", corpusID)
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
	rawUrl2, err := url.JoinPath(mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "freqs-by-year-streamed", corpusID)
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

// mostFreqLemmaQuery loads freq. distribution of lemmas matching the query q
// in corpusID and returns a query searching for the most frequent one.
// In case no lemma is found, the original query is returned.
func (mp *MQueryProxy) mostFreqLemmaQuery(
	origReq *http.Request,
	reqProps guard.ReqEvaluation,
	corpusID string,
	q string,
) (string, error) {
	lmArgs := lemmaFreqDistArgs{
		q:         q,
		attr:      "lemma",
		matchCase: 0,
		maxItems:  10,
		flimit:    5,
	}
	lemmaFreqURL, err := mp.createLemmaFreqsURL(corpusID, lmArgs)
	if err != nil {
		return "", fmt.Errorf("failed to create lemma freqs URL for corpus %s: %w", corpusID, err)
	}
	req := *origReq
	req.URL = lemmaFreqURL
	req.Method = "GET"
	serviceResp := mp.HandleRequest(&req, reqProps, true)
	if serviceResp.Error() != nil {
		return "", fmt.Errorf("failed to get lemma freqs for corpus %s: %w", corpusID, serviceResp.Error())
	}

	var lemmaData lemmaFreqResponse
	respBody, err := serviceResp.ExportResponse()
	if err != nil {
		return "", fmt.Errorf("failed to get lemma freqs for corpus %s: %w", corpusID, err)
	}
	if err := sonic.Unmarshal(respBody, &lemmaData); err != nil {
		return "", fmt.Errorf("failed to decode lemma freqs for corpus %s: %w", corpusID, err)
	}
	if lemmaData.Error != "" {
		return "", fmt.Errorf("failed to get lemma freqs for corpus %s: %s", corpusID, lemmaData.Error)
	}
	if len(lemmaData.Freqs) == 0 {
		return q, nil
	}
	return fmt.Sprintf(`[lemma="%s"]`, lemmaData.Freqs[0].Word), nil
}

// ------------------------------------

func (mp *MQueryProxy) TimeDistAltWord(ctx *gin.Context) {
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

	corpusID := ctx.Query("corpname")
	// first, load freq dist. by lemma and select the most freq. one
	q, err := mp.mostFreqLemmaQuery(ctx.Request, reqProps, corpusID, ctx.Query("q"))
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to process: %w", err), http.StatusInternalServerError)
		return
	}

	// then call "classic" streamed time dist

	flimit, err := strconv.Atoi(ctx.DefaultQuery("flimit", "10"))
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("flimit arg: %w", err), http.StatusBadRequest)
		return
	}

	fromYear, ok := unireq.GetURLIntArgOrFail(ctx, "fromYear", 0)
	if !ok {
		return
	}
	toYear, ok := unireq.GetURLIntArgOrFail(ctx, "toYear", 0)
	if !ok {
		return
	}

	url2, err := mp.createTimeDistURL(
		corpusID,
		streamedFreqDistArgs{
			q:        q,
			attr:     ctx.Query("attr"),
			fcrit:    ctx.Query("fcrit"),
			flimit:   flimit,
			maxItems: 100, // TODO
			event:    ctx.Query("event"),
			fromYear: fromYear,
			toYear:   toYear,
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
	statusCode = resp2.Response().GetStatusCode()

	ctx.Writer.Header().Set("Content-Type", resp2.Response().GetHeaders().Get("Content-Type"))
	ctx.Writer.WriteHeader(resp2.Response().GetStatusCode())

	resp2.WriteResponse(ctx.Writer)

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}

// -----

type mergeStreamedFreqDistArgs struct {
	MaxItems int    `json:"maxItems"`
	Event    string `json:"event"`

	Corpora []struct {
		Corpname string `json:"corpname"`
		Q        string `json:"q"`
		Attr     string `json:"attr"`
		Fcrit    string `json:"fcrit"`
		Flimit   int    `json:"flimit"`
		FromYear int    `json:"fromYear"`
		ToYear   int    `json:"toYear"`
	} `json:"corpora"`
}

// corpusYearEvent carries a single complete SSE event read from one of the
// per-corpus backend streams (or an error terminating that stream) to the
// fan-in consumer below.
type corpusYearEvent struct {
	corpusIdx int
	data      []byte // raw "event: ...\ndata: ...\n\n" chunk, as read from the backend
	err       error
}

// corpusStreamPayload mirrors the two JSON shapes the mquery backend sends as
// the "data:" part of a freqs-by-year-streamed SSE event (see mquery's
// corpus/handlers/ttchunked.go StreamData/streamingError): either a
// (cumulative, not incremental) frequency chunk with an Entries payload, or a
// bare {"error": "..."} frame.
type corpusStreamPayload struct {
	Entries     *partialFreqResponse `json:"entries,omitempty"`
	ChunkNum    int                  `json:"chunkNum,omitempty"`
	TotalChunks int                  `json:"totalChunks,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// yearStreamPart is the per-corpus slot of the merged response sent to our
// own client, analogous to partialFreqResponse in mergeFreqsResponse (see
// mergefreq.go) but keeping the streaming chunk bookkeeping too.
type yearStreamPart struct {
	Corpname    string               `json:"corpname"`
	Subcname    string               `json:"subcname"`
	ChunkNum    int                  `json:"chunkNum,omitempty"`
	TotalChunks int                  `json:"totalChunks,omitempty"`
	Entries     *partialFreqResponse `json:"entries,omitempty"`
	Error       string               `json:"error,omitempty"`
}

type mergedParts []*yearStreamPart

func (mp mergedParts) firstError() string {
	for _, v := range mp {
		if v.Error != "" {
			return v.Error
		}
	}
	return ""
}

// mergedYearStreamResponse is re-sent (with growing/updated Parts) every time
// any single corpus stream produces a new chunk.
type mergedYearStreamResponse struct {
	Error string      `json:"error"`
	Parts mergedParts `json:"parts"`
}

// extractSSEEventData pulls the concatenated payload of all "data:" lines out
// of a single raw SSE frame produced by fetchCorpusYearStream, ignoring any
// "event:"/"id:"/"retry:" lines, per the SSE framing spec.
func extractSSEEventData(frame []byte) []byte {
	var lines [][]byte
	for _, line := range bytes.Split(frame, []byte("\n")) {
		rest, ok := bytes.CutPrefix(line, []byte("data:"))
		if !ok {
			continue
		}
		lines = append(lines, bytes.TrimPrefix(rest, []byte(" ")))
	}
	return bytes.Join(lines, []byte("\n"))
}
