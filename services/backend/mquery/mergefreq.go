// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/interop"
	"apiguard/proxy"
	"apiguard/reporting"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (mp *MQueryProxy) MergeFreqs(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	// we must prepare request's body for repeated reading
	var err error
	ctx.Request.Body, err = proxy.NewReReader(ctx.Request.Body)
	if err != nil {
		http.Error(ctx.Writer, "Failed to process merge-freqs args", http.StatusBadRequest)
		return
	}

	defer mp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	rawReq1Body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		http.Error(ctx.Writer, "Failed to process merge-freqs args", http.StatusBadRequest)
		return
	}
	var args mergeFreqsArgs
	if err := sonic.Unmarshal(rawReq1Body, &args); err != nil {
		http.Error(ctx.Writer, "Failed to process merge-freqs args", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(ctx.Request.URL.Path, mp.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to get merge freqs - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := mp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err = mp.Guard().DetermineTrueUserID(ctx.Request)
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
		log.Error().Err(reqProps.Error).Msgf("failed to get merge freqs - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rt0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	var tileID int

	if mp.EnvironConf().IsStreamingMode {
		_, err = fmt.Fprint(ctx.Writer, "{}\n\n")
		if err != nil {
			log.Error().Err(err).Msg("failed to write SSE response")
		}
		var err error
		tileID, err = interop.TileIdFromReq(ctx.Request)
		if err != nil {
			http.Error(
				ctx.Writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	// For this endpoint/action we cache the whole request
	// (i.e. all sub-requests in a single shot). This requires
	// more low-level access to cache functions.
	cacheApplCookies := make([]string, 0, 2)
	if mp.Conf().CachingPerSession {
		cacheApplCookies = append(
			cacheApplCookies,
			mp.EnvironConf().CNCAuthCookie,
			mp.Conf().FrontendSessionCookieName,
		)
	}
	// options must be used for both read and write
	cachingOpts := []func(*proxy.CacheEntryOptions){
		proxy.CachingWithCookies(cacheApplCookies),
		proxy.CachingWithCacheablePOST(),
	}
	respProc := mp.FromCache(ctx.Request, cachingOpts...)

	if respProc.IsCacheHit() {
		cached = true
		respProc.WriteResponse(ctx.Writer)

	} else {
		cached = false
		data := mergeFreqsResponse{
			Parts: make([]*partialFreqResponse, 0, len(args.URLs)),
		}
		toCache := new(bytes.Buffer)
		sseEvent := ""
		if mp.EnvironConf().IsStreamingMode {
			sseEvent = fmt.Sprintf(" DataTile-%d.%d", tileID, 0)
		}

		for _, u := range args.URLs {
			req := *ctx.Request
			parsedURL, err := url.Parse(u)
			if err != nil {
				log.Error().Err(err).Msgf("failed to parse URL: %s", u)
				http.Error(ctx.Writer, "Invalid URL", http.StatusBadRequest)
				return
			}
			req.URL = parsedURL
			req.Method = "GET"
			resp := mp.HandleRequest(&req, reqProps, false)

			if resp.Error() != nil {
				log.Error().Err(resp.Error()).Msgf("failed to to get partial freqs %s", ctx.Request.URL.Path)
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial freqs: %s", resp.Error()),
					http.StatusInternalServerError,
				)
				return
			}

			body, err := resp.ExportResponse()
			if err != nil {
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial freqs: %s", err),
					http.StatusInternalServerError,
				)
			}
			var freqPart partialFreqResponse
			if err := sonic.Unmarshal(body, &freqPart); err != nil {
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial freqs: %s", err),
					http.StatusInternalServerError,
				)
				return
			}
			data.Parts = append(data.Parts, &freqPart)
			if data.Error == "" && freqPart.Error != "" {
				data.Error = freqPart.Error
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				log.Error().Err(err).Msg("failed to prepare EventSource data")
				return
			}

			output := fmt.Sprintf("event:%s\ndata: %s\n\n", sseEvent, jsonData)
			_, err = ctx.Writer.WriteString(output)
			toCache.WriteString(output)
			if err != nil {
				// not much we can do here
				log.Error().Err(err).Msg("failed to write EventSource data")
				return
			}
		}
		mp.ToCache(
			ctx.Request,
			proxy.CacheEntry{
				Status: http.StatusOK,
				Data:   toCache.Bytes(),
				Headers: http.Header{
					"Content-Type": []string{"text/event-stream"},
				},
			},
			cachingOpts...,
		)
	}

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}
