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
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
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
	fromYear string
	toYear   string
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
	if sfargs.fromYear != "" {
		q.Add("fromYear", sfargs.fromYear)
	}
	if sfargs.toYear != "" {
		q.Add("toYear", sfargs.toYear)
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
	serviceResp := mp.HandleRequest(&req1, reqProps, true)

	var lemmaData lemmaFreqResponse
	resp1Body, err := serviceResp.ExportResponse()
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
			fcrit:    ctx.Query("fcrit"),
			flimit:   flimit,
			maxItems: 100, // TODO
			event:    ctx.Query("event"),
			fromYear: ctx.Query("fromYear"),
			toYear:   ctx.Query("toYear"),
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
		FromYear string `json:"fromYear"`
		ToYear   string `json:"toYear"`
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
	ChunkNum    int                  `json:"chunkNum,omitempty"`
	TotalChunks int                  `json:"totalChunks,omitempty"`
	Entries     *partialFreqResponse `json:"entries,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// mergedYearStreamResponse is re-sent (with growing/updated Parts) every time
// any single corpus stream produces a new chunk.
type mergedYearStreamResponse struct {
	Parts []*yearStreamPart `json:"parts"`
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

// fetchCorpusYearStream is run once per queried corpus/backend. It owns the
// backend request/response for that single corpus and only ever *sends* on
// out - it must never write to ctx.Writer directly, since ResponseWriter is
// not safe for concurrent use and only the fan-in consumer goroutine writes
// to it.
func (mp *MQueryProxy) fetchCorpusYearStream(
	ctx *gin.Context,
	reqProps guard.ReqEvaluation,
	corpusIdx int,
	corpname string,
	corpusArgs streamedFreqDistArgs,
	out chan<- corpusYearEvent,
) {
	// send delivers ev unless the client has gone away, in which case it
	// gives up so this goroutine does not block forever on a full channel
	// whose consumer already stopped reading.
	send := func(ev corpusYearEvent) bool {
		select {
		case out <- ev:
			return true
		case <-ctx.Request.Context().Done():
			return false
		}
	}

	reqURL, err := mp.createTimeDistURL(corpname, corpusArgs)
	if err != nil {
		send(corpusYearEvent{
			corpusIdx: corpusIdx,
			err:       fmt.Errorf("failed to create time dist url for corpus %s: %w", corpname, err),
		})
		return
	}

	req := *ctx.Request
	req.URL = reqURL
	req.Method = "GET"

	respProc := mp.MakeStreamRequest(&req, reqProps)
	if respProc.Error() != nil {
		send(corpusYearEvent{corpusIdx: corpusIdx, err: respProc.Error()})
		return
	}

	backendResp := respProc.Response()
	if backendResp == nil || backendResp.GetBodyReader() == nil {
		send(corpusYearEvent{
			corpusIdx: corpusIdx,
			err:       fmt.Errorf("no response body for corpus %s", corpname),
		})
		return
	}
	defer backendResp.CloseBodyReader()

	// Same event framing as ThroughCacheResponse.writeSSEResponse: lines are
	// buffered until a blank line terminates a complete SSE event, which is
	// then forwarded as one unit.
	scanner := bufio.NewScanner(backendResp.GetBodyReader())
	var eventChunk []string
	flushChunk := func() bool {
		if len(eventChunk) == 0 {
			return true
		}
		completeEvent := []byte(strings.Join(eventChunk, "\n") + "\n\n")
		eventChunk = eventChunk[:0]
		return send(corpusYearEvent{corpusIdx: corpusIdx, data: completeEvent})
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if !flushChunk() {
				return
			}
		} else {
			eventChunk = append(eventChunk, line)
		}
	}
	if err := scanner.Err(); err != nil {
		send(corpusYearEvent{
			corpusIdx: corpusIdx,
			err:       fmt.Errorf("failed to read stream for corpus %s: %w", corpname, err),
		})
		return
	}
	flushChunk()
}

func (mp *MQueryProxy) MergeFreqsByYearStreamed(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, firstPartyAPICall bool
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	var args mergeStreamedFreqDistArgs
	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to decode args: %w", err), http.StatusInternalServerError)
		return
	}

	defer mp.LogRequest(ctx, &humanID, &firstPartyAPICall, &cached, t0)

	// guard request

	if !strings.HasPrefix(ctx.Request.URL.Path, mp.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to get merge freqs by year - invalid path detected")
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
		log.Error().Err(reqProps.Error).Msgf("failed to get merge freqs by year - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// process request

	rt0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.WriteHeader(http.StatusOK)

	// events is the fan-in channel: N producer goroutines (one per corpus)
	// send into it, a single consumer (this goroutine, below) reads from it
	// and is the only one allowed to write to ctx.Writer.
	events := make(chan corpusYearEvent, len(args.Corpora))

	var wg sync.WaitGroup
	for i, corpusArgs := range args.Corpora {
		wg.Add(1)
		go func(idx int, corpname string, cArgs streamedFreqDistArgs) {
			defer wg.Done()
			mp.fetchCorpusYearStream(ctx, reqProps, idx, corpname, cArgs, events)
		}(i, corpusArgs.Corpname, streamedFreqDistArgs{
			q:        corpusArgs.Q,
			attr:     corpusArgs.Attr,
			fcrit:    corpusArgs.Fcrit,
			flimit:   corpusArgs.Flimit,
			maxItems: args.MaxItems,
			event:    args.Event,
			fromYear: corpusArgs.FromYear,
			toYear:   corpusArgs.ToYear,
		})
	}

	// closer: once all producers are done (success or error), close the
	// channel so the fan-in loop below terminates. This must run in its own
	// goroutine so it does not block producers from being drained concurrently.
	go func() {
		wg.Wait()
		close(events)
	}()

	// parts holds the latest known state per corpus. Each incoming chunk from
	// a given corpus is cumulative (not a delta), so a new chunk simply
	// replaces that corpus's slot; the merged response re-sent below always
	// reflects the latest chunk seen from every corpus so far.
	parts := make([]*yearStreamPart, len(args.Corpora))
	for i, corpusArgs := range args.Corpora {
		parts[i] = &yearStreamPart{Corpname: corpusArgs.Corpname}
	}

	// fan-in loop - the only place allowed to write to ctx.Writer.
	flusher, _ := ctx.Writer.(http.Flusher)
	for ev := range events {
		part := parts[ev.corpusIdx]
		if ev.err != nil {
			part.Error = ev.err.Error()

		} else {
			var payload corpusStreamPayload
			if err := sonic.Unmarshal(extractSSEEventData(ev.data), &payload); err != nil {
				log.Error().Err(err).
					Str("corpname", part.Corpname).
					Msg("failed to parse upstream freqs-by-year-streamed event")
				part.Error = fmt.Sprintf("failed to parse upstream event: %s", err)

			} else if payload.Error != "" {
				part.Error = payload.Error

			} else {
				part.ChunkNum = payload.ChunkNum
				part.TotalChunks = payload.TotalChunks
				part.Entries = payload.Entries
				part.Error = ""
			}
		}

		jsonData, err := sonic.Marshal(mergedYearStreamResponse{Parts: parts})
		if err != nil {
			log.Error().Err(err).Msg("failed to prepare merged EventSource data")
			continue
		}
		var output string
		if args.Event != "" {
			output = fmt.Sprintf("event: %s\ndata: %s\n\n", args.Event, jsonData)
		} else {
			output = fmt.Sprintf("data: %s\n\n", jsonData)
		}
		if _, err := ctx.Writer.WriteString(output); err != nil {
			// client is very likely gone - nothing more we can usefully do;
			// the still-running producers will unblock once they notice
			// ctx.Request.Context() has been cancelled.
			log.Error().Err(err).Msg("failed to write merged EventSource data")
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}
