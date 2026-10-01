// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
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
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/reporting"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// fetchCorpusYearStream is run once per queried corpus/backend. It owns the
// backend request/response for that single corpus and only ever *sends* on
// out - it must never write to ctx.Writer directly, since ResponseWriter is
// not safe for concurrent use and only the fan-in consumer goroutine writes
// to it.
func (mp *MQueryProxy) fetchCorpusYearStream(
	ctx *gin.Context,
	done <-chan struct{},
	reqProps guard.ReqEvaluation,
	corpusIdx int,
	corpname string,
	corpusArgs streamedFreqDistArgs,
	useMostFreqLemma bool,
	out chan<- corpusYearEvent,
) {
	// send delivers ev unless the consumer has stopped reading (client gone,
	// handler returned), in which case it gives up so this goroutine does not
	// block forever on a full channel.
	send := func(ev corpusYearEvent) bool {
		select {
		case out <- ev:
			return true
		case <-done:
			return false
		}
	}

	if useMostFreqLemma {
		q, err := mp.mostFreqLemmaQuery(ctx.Request, reqProps, corpname, corpusArgs.q)
		if err != nil {
			send(corpusYearEvent{corpusIdx: corpusIdx, err: err})
			return
		}
		corpusArgs.q = q
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

	// In case of an early failure (e.g. invalid args), the backend does not
	// open the stream at all and responds with a plain JSON error instead.
	if status := backendResp.GetStatusCode(); status < 200 || status >= 300 {
		send(corpusYearEvent{
			corpusIdx: corpusIdx,
			err:       backendErrorFromBody(backendResp.GetBodyReader(), status),
		})
		return
	}

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

// backendErrorFromBody extracts an error message from a non-streamed
// backend error response (e.g. `{"code":422,"error":"..."}`).
func backendErrorFromBody(body io.Reader, status int) error {
	rawBody, err := io.ReadAll(io.LimitReader(body, 64*1024))
	if err != nil {
		return fmt.Errorf("backend responded with status %d (failed to read body: %w)", status, err)
	}
	var errResp struct {
		Error string `json:"error"`
	}
	if err := sonic.Unmarshal(rawBody, &errResp); err != nil || errResp.Error == "" {
		return fmt.Errorf("backend responded with status %d", status)
	}
	return fmt.Errorf("%s", errResp.Error)
}

func (mp *MQueryProxy) MergeFreqsByYearStreamed(ctx *gin.Context) {
	mp.mergeTimeDistStreamed(ctx, false)
}

// MergeTimeDistAltWord is a multi-corpus variant of TimeDistAltWord. For each
// corpus, the most frequent lemma matching the corpus' query is found first
// and the streamed time distribution is then calculated for that lemma.
// Data from all the corpora are merged the same way as in MergeFreqsByYearStreamed.
func (mp *MQueryProxy) MergeTimeDistAltWord(ctx *gin.Context) {
	mp.mergeTimeDistStreamed(ctx, true)
}

// mergeTimeDistStreamed queries freqs-by-year-streamed for each corpus
// in parallel and merges the streams into a single one. If useMostFreqLemma
// is true, the query for each corpus is first replaced by a query for
// the most frequent lemma matching the original query (see TimeDistAltWord).
func (mp *MQueryProxy) mergeTimeDistStreamed(ctx *gin.Context, useMostFreqLemma bool) {
	var userID, humanID common.UserID
	var cached, firstPartyAPICall bool
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	var args mergeStreamedFreqDistArgs
	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to decode args: %w", err), http.StatusUnprocessableEntity)
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

	// In the streaming mode (wagstream), the request is created internally
	// and its context is never cancelled, so we need our own cancellation
	// to release producers in case we leave the fan-in loop early.
	procCtx, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()

	var wg sync.WaitGroup
	for i, corpusArgs := range args.Corpora {
		wg.Add(1)
		go func(idx int, corpname string, cArgs streamedFreqDistArgs) {
			defer wg.Done()
			mp.fetchCorpusYearStream(
				ctx, procCtx.Done(), reqProps, idx, corpname, cArgs, useMostFreqLemma, events)
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
	parts := make(mergedParts, len(args.Corpora))
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
			eventData := extractSSEEventData(ev.data)
			if len(eventData) == 0 {
				continue // e.g. keep-alive or other frame without data
			}
			var payload corpusStreamPayload
			if err := sonic.Unmarshal(eventData, &payload); err != nil {
				log.Error().Err(err).
					Str("corpname", part.Corpname).
					Str("rawFrame", string(ev.data)).
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

		jsonData, err := sonic.Marshal(mergedYearStreamResponse{
			Parts: parts,
			Error: parts.firstError(),
		})
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
			// the still-running producers will unblock once the deferred
			// cancel() closes procCtx.
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
