// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wss

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/cnc"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

//

type WSServerProxy struct {
	*cnc.Proxy
	httpEngine http.Handler
	conf       *cnc.ProxyConf
}

func (wssProxy *WSServerProxy) CollocationsTT(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	var statusCode int
	t0 := time.Now().In(wssProxy.GlobalCtx().TimezoneLocation)

	// we must prepare request's body for repeated reading
	var err error
	ctx.Request.Body, err = proxy.NewReReader(ctx.Request.Body)
	if err != nil {
		http.Error(ctx.Writer, "Failed to process tt-collocations args", http.StatusBadRequest)
		return
	}

	defer wssProxy.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	rawReq1Body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		http.Error(ctx.Writer, "Failed to process tt-collocations args", http.StatusBadRequest)
		return
	}
	var args ttCollsFreqsArgs
	if err := sonic.Unmarshal(rawReq1Body, &args); err != nil {
		http.Error(ctx.Writer, "Failed to process tt-collocations args", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(ctx.Request.URL.Path, wssProxy.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to get merge freqs - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := wssProxy.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err = wssProxy.Guard().DetermineTrueUserID(ctx.Request)
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
		ctx.Writer, ctx.Request, wssProxy.EnvironConf().ReadTimeoutSecs, wssProxy.Guard(), clientID,
	); err != nil {
		return
	}

	if err := wssProxy.ProcessReqHeaders(
		ctx, humanID, userID, &indirectAPICall,
	); err != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to get tt-collocations - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rt0 := time.Now().In(wssProxy.GlobalCtx().TimezoneLocation)

	// the sub-requests processing is a bit complicated here as we want
	// to cache finished data at once (and not the particular responses)

	// let's try cache for whole request (i.e. all sub-requests in a single shot)
	cacheApplCookies := make([]string, 0, 2)
	if wssProxy.conf.CachingPerSession {
		cacheApplCookies = append(
			cacheApplCookies,
			wssProxy.EnvironConf().CNCAuthCookie,
			wssProxy.conf.FrontendSessionCookieName,
		)
	}
	// options must be used for both read and write
	cachingOpts := []func(*proxy.CacheEntryOptions){
		proxy.CachingWithCookies(cacheApplCookies),
		proxy.CachingWithCacheablePOST(),
	}
	respProc := wssProxy.FromCache(ctx.Request, cachingOpts...)

	// TODO this is debatable:
	// here we write empty data so the stream handler
	// fills in data for "data: ..." and then we continue
	// writing true data with both "event" and "data"
	// But we have to make sure this cannot fail in strange ways
	// This also means, client has to know how to handle the {} response
	if wssProxy.EnvironConf().IsStreamingMode {
		_, err = fmt.Fprint(ctx.Writer, "{}\n\n")
		if err != nil {
			log.Error().Err(err).Msg("failed to write response")
			// TODO
		}
	}

	if !respProc.IsCacheMiss() {
		cached = false
		respProc.WriteResponse(ctx.Writer)

	} else {
		cached = true
		data := streamResponse{
			Parts: make(map[string]collResponse),
		}

		toCache := new(bytes.Buffer)
		statusCodes := make(proxy.MultiStatusCode, len(args.TextTypes))
		if err != nil {
			http.Error(
				ctx.Writer,
				fmt.Sprintf("Failed to process tt-collocations args: %s", err),
				http.StatusBadRequest,
			)
			return
		}

		sseEvent := ""
		if wssProxy.EnvironConf().IsStreamingMode {
			sseEvent = fmt.Sprintf(" DataTile-%d.%d", args.TileID, 0)
		}

		for reqIdx, textType := range args.TextTypes {
			req := *ctx.Request
			var reqURL *url.URL
			if args.PoS != "" {
				reqURL = wssProxy.BackendURL.JoinPath(
					wssProxy.EnvironConf().ServicePath, "dataset", args.Dataset, "collocations", args.Word, args.PoS)

			} else {
				reqURL = wssProxy.BackendURL.JoinPath(
					wssProxy.EnvironConf().ServicePath, "dataset", args.Dataset, "collocations", args.Word)
			}
			urlArgs := make(url.Values)
			urlArgs.Add("tt", textType)
			urlArgs.Add("limit", strconv.Itoa(args.Limit))
			reqURL.RawQuery = urlArgs.Encode()
			req.Body = ctx.Request.Body
			req.URL = reqURL
			req.Method = "GET"
			resp := wssProxy.HandleRequest(&req, reqProps, false)
			statusCodes[reqIdx] = resp.Response().GetStatusCode()

			if resp.Error() != nil {
				log.Error().Err(resp.Error()).Msgf("failed to to get partial tt-colls %s", reqURL.String())
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial tt-colls: %s", resp.Error()),
					http.StatusInternalServerError,
				)
				return
			}

			body, err := resp.ExportResponse()
			if err != nil {
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial tt-colls: %s", err),
					http.StatusInternalServerError,
				)
			}
			var ttCollPart collResponse
			if err := sonic.Unmarshal(body, &ttCollPart); err != nil {
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to get partial freqs: %s", err),
					http.StatusInternalServerError,
				)
				return
			}
			data.Parts[textType] = ttCollPart
			if data.Error == "" && ttCollPart.Error != "" {
				data.Error = ttCollPart.Error
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
		wssProxy.ToCache(
			ctx.Request,
			proxy.CacheEntry{
				Status: statusCodes.Result(),
				Data:   toCache.Bytes(),
				Headers: http.Header{
					"Content-Type": []string{"text/event-stream"},
				},
			},
			cachingOpts...,
		)

	}

	wssProxy.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(wssProxy.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  wssProxy.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}

func NewWSServerProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	httpEngine http.Handler,
	reqCounter chan<- guard.RequestInfo,
) (*WSServerProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQuery proxy: %w", err)
	}
	return &WSServerProxy{
		Proxy:      proxy,
		httpEngine: httpEngine,
		conf:       conf,
	}, nil
}
