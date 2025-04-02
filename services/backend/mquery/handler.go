// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/reporting"
	"apiguard/services/cnc"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type MQueryProxy struct {
	*cnc.CoreProxy
}

func (mp *MQueryProxy) MergeFreqs(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	var statusCode int
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

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

	cached = true
	data := mergeFreqsResponse{
		Parts: make([]*partialFreqResponse, 0, len(args.URLS)),
	}
	for _, u := range args.URLS {
		req := *ctx.Request
		parsedURL, err := url.Parse(u)
		if err != nil {
			log.Error().Err(err).Msgf("failed to parse URL: %s", u)
			http.Error(ctx.Writer, "Invalid URL", http.StatusBadRequest)
			return
		}
		req.URL = parsedURL
		req.Method = "GET"
		resp := mp.MakeRequest(&req, reqProps)
		if resp.GetError() != nil {
			log.Error().Err(resp.GetError()).Msgf("failed to to get partial freqs %s", ctx.Request.URL.Path)
			http.Error(
				ctx.Writer,
				fmt.Sprintf("failed to get partial freqs: %s", resp.GetError()),
				http.StatusInternalServerError,
			)
			return
		}

		body := resp.GetBody()
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

		for k, v := range resp.GetHeaders() {
			ctx.Writer.Header().Set(k, v[0])
		}
		statusCode = resp.GetStatusCode()
		cached = cached && resp.IsCached()
	}

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})

	ctx.Writer.WriteHeader(statusCode)
	uniresp.WriteJSONResponse(ctx.Writer, data)
}

func (mp *MQueryProxy) Speeches(ctx *gin.Context) {
	uniresp.RespondWithErrorJSON(
		ctx, fmt.Errorf("not implemented yet"), http.StatusNotImplemented,
	)
}

func NewMQueryProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*MQueryProxy, error) {
	proxy, err := cnc.NewCoreProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQuery proxy: %w", err)
	}
	return &MQueryProxy{
		CoreProxy: proxy,
	}, nil
}
