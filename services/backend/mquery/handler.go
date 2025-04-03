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
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
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

func (mp *MQueryProxy) createConcMqueryURL(
	ctx *gin.Context,
	corpusID string,
	reqArgs concordanceRequestArgs,
) (*url.URL, error) {
	url1 := ctx.Request.URL
	baseURL := fmt.Sprintf("%s://%s", url1.Scheme, url1.Host)
	rawUrl2, err := url.JoinPath(baseURL, mp.EnvironConf().ServicePath, "concordance", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = reqArgs.ToURLQuery()
	return url2, nil
}

func (mp *MQueryProxy) createTokenContextMqueryURL(
	ctx *gin.Context,
	corpusID string,
	reqArgs tokenContextRequestArgs,
) (*url.URL, error) {
	url1 := ctx.Request.URL
	baseURL := fmt.Sprintf("%s://%s", url1.Scheme, url1.Host)
	rawUrl2, err := url.JoinPath(baseURL, mp.EnvironConf().ServicePath, "token-context", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create token-context URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create token-context URL: %w", err)
	}
	url2.RawQuery = reqArgs.ToURLQuery()
	return url2, nil
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
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	var statusCode int
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	defer mp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

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

	rt0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)
	args := speechesArgs{
		Corpname:  ctx.Request.URL.Query().Get("corpname"),
		Subcorpus: ctx.Request.URL.Query().Get("subcorpus"),
		Query:     ctx.Request.URL.Query().Get("query"),
	}
	if leftCtx := ctx.Request.URL.Query().Get("leftCtx"); leftCtx != "" {
		if i, err := strconv.Atoi(leftCtx); err == nil {
			args.LeftCtx = i
		} else {
			uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Errorf("failed to parse leftCtx: %s", err), http.StatusBadRequest)
			return
		}
	}
	if rightCtx := ctx.Request.URL.Query().Get("rightCtx"); rightCtx != "" {
		if i, err := strconv.Atoi(rightCtx); err == nil {
			args.RightCtx = i
		} else {
			uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Errorf("failed to parse rightCtx: %s", err), http.StatusBadRequest)
			return
		}
	}
	if structs, ok := ctx.Request.URL.Query()["struct"]; ok {
		args.Structs = structs
	}

	req1URL, err := mp.createConcMqueryURL(
		ctx,
		args.Corpname,
		concordanceRequestArgs{
			Q:            args.Query,
			Subcorpus:    args.Subcorpus,
			ContextWidth: 0,
			CollRange:    "0,0",
		},
	)
	if err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	req1 := *ctx.Request
	req1.URL = req1URL
	req1.Method = "GET"
	serviceResp := mp.MakeRequest(&req1, reqProps)
	statusCode = serviceResp.GetStatusCode()
	if err := serviceResp.GetError(); err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, err, statusCode)
		return
	}
	resp1Body := serviceResp.GetBody()
	var resp1 concordanceResponse
	if err := sonic.Unmarshal(resp1Body, &resp1); err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	if resp1.Error != "" {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, resp1.Error, statusCode)
		return
	}

	pos, err := strconv.Atoi(resp1.Lines[rand.Intn(len(resp1.Lines))].Ref[1:])
	if err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	req2URL, err := mp.createTokenContextMqueryURL(
		ctx,
		args.Corpname,
		tokenContextRequestArgs{
			Pos:      pos,
			LeftCtx:  args.LeftCtx,
			RightCtx: args.RightCtx,
			Structs:  args.Structs,
		},
	)
	if err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query token context: %s", err), http.StatusInternalServerError)
		return
	}
	req2 := *ctx.Request
	req2.URL = req2URL
	req2.Method = "GET"
	serviceResp = mp.MakeRequest(&req2, reqProps)
	statusCode = serviceResp.GetStatusCode()
	if err := serviceResp.GetError(); err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, err, statusCode)
		return
	}
	resp2Body := serviceResp.GetBody()
	var resp2 tokenContextResponse
	if err := sonic.Unmarshal(resp2Body, &resp2); err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query token context: %s", err), http.StatusInternalServerError)
		return
	}
	if resp2.Error != "" {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, resp2.Error, statusCode)
		return
	}

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})

	uniresp.WriteJSONResponse(ctx.Writer, resp2)
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
