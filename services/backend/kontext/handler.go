// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/reporting"
	"apiguard/services/cnc"
	"bytes"
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

type KonTextProxy struct {
	*cnc.Proxy
	conf *Conf
}

// createConcViewURL creates a concordance view URL based on submit args
// and existing queryID. The ctx.Request is expected to be the request
// used to submit the query. This method will use it to generate proper
// URL for viewing the concordance.
func (kp *KonTextProxy) createConcViewURL(
	ctx *gin.Context,
	submitArgs querySubmitArgs,
	queryID string,
) (*url.URL, error) {
	url1 := ctx.Request.URL
	rawUrl2, err := url.JoinPath(url1.Host, kp.EnvironConf().ServicePath, "view")
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create submit URL: %w", err)
	}
	rawUrl2 = fmt.Sprintf("%s://%s", url1.Scheme, rawUrl2)
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create submit URL: %w", err)
	}
	var viewArgs viewActionArgs
	viewArgs.Attr = strings.Join(submitArgs.Attrs, ",")
	viewArgs.AttrVmode = submitArgs.AttrVmode
	viewArgs.Format = "json"
	viewArgs.Fromp = strconv.Itoa(submitArgs.Fromp)
	viewArgs.KWICLeftCtx = strconv.Itoa(submitArgs.KWICLeftCtx)
	viewArgs.KWICRightCtx = strconv.Itoa(submitArgs.KWICRightCtx)
	viewArgs.Maincorp = submitArgs.Maincorp
	viewArgs.Pagesize = strconv.Itoa(submitArgs.Pagesize)
	viewArgs.Viewmode = submitArgs.Viewmode
	viewArgs.Q = fmt.Sprintf("~%s", queryID)
	viewArgs.Refs = strings.Join(submitArgs.Refs, ",")
	url2.RawQuery = viewArgs.ToURLQuery()
	return url2, nil
}

// QuerySubmitAndView is a handler which takes a KonText query_submit
// request and returns a matching concordance directly, i.e. the API
// user does not have to do the two-request action 'query_submit' + 'view'
// as the handler will do it for them.
// The method is used either when KonText service is configured with
// the `useSimplifiedConcReq` configuration or in case APIGuard runs in
// the "streaming" mode.
// This is useful mostly for the data streaming mode where we want
// avoid those chained API calls as much as possible (as otherwise, we would
// be forced to perform the subsequent actions out of the data stream).
func (kp *KonTextProxy) QuerySubmitAndView(ctx *gin.Context) {

	if !kp.conf.UseSimplifiedConcReq && !kp.EnvironConf().IsStreamingMode {
		kp.AnyPath(ctx)
		return
	}

	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	t0 := time.Now().In(kp.GlobalCtx().TimezoneLocation)

	defer kp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	rawReq1Body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		http.Error(ctx.Writer, "Failed to process submit args", http.StatusBadRequest)
		return
	}
	var submitArgs querySubmitArgs
	if err := sonic.Unmarshal(rawReq1Body, &submitArgs); err != nil {
		http.Error(ctx.Writer, "Failed to process submit args", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(ctx.Request.URL.Path, kp.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to proxy request - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := kp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err = kp.Guard().DetermineTrueUserID(ctx.Request)
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
		ctx.Writer, ctx.Request, kp.EnvironConf().ReadTimeoutSecs, kp.Guard(), clientID,
	); err != nil {
		return
	}

	if err := kp.ProcessReqHeaders(
		ctx, humanID, userID, &indirectAPICall,
	); err != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to proxy query_submit - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rt0 := time.Now().In(kp.GlobalCtx().TimezoneLocation)

	req1 := *ctx.Request
	req1.Body = io.NopCloser(bytes.NewBuffer(rawReq1Body))

	serviceResp := kp.MakeCacheablePOSTRequest(&req1, reqProps, rawReq1Body)
	cached = serviceResp.IsCacheHit()
	kp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(kp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.Response().GetStatusCode(),
		Service:  kp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
	if serviceResp.Error() != nil {
		log.Error().Err(serviceResp.Error()).Msgf("failed to proxy query_submit %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy query_submit: %s", serviceResp.Error()),
			http.StatusInternalServerError,
		)
		return
	}
	defer serviceResp.Response().GetBodyReader().Close()
	resp1Body, err := io.ReadAll(serviceResp.Response().GetBodyReader())
	if err != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy query_submit: %s", serviceResp.Response().Error()),
			http.StatusInternalServerError,
		)
		return
	}
	var resp1 querySubmitResponse
	if err := sonic.Unmarshal(resp1Body, &resp1); err != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy query_submit: %s", err),
			http.StatusInternalServerError,
		)
		return
	}
	// now we create the 2nd request - /view
	req2URL, err := kp.createConcViewURL(ctx, submitArgs, resp1.ConcPersistenceOpID)
	if err != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy query_submit: %s", err),
			http.StatusInternalServerError,
		)
		return
	}
	req2 := *ctx.Request
	req2.URL = req2URL
	req2.Method = "GET"
	serviceResp = kp.HandleRequest(&req2, reqProps, true)

	for k, v := range serviceResp.Response().GetHeaders() {
		ctx.Writer.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	ctx.Writer.WriteHeader(serviceResp.Response().GetStatusCode())
	defer serviceResp.Response().GetBodyReader().Close()

	body2, err := serviceResp.ExportResponse()
	if err != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy query_submit: %s", err),
			http.StatusInternalServerError,
		)
		return
	}
	ctx.Writer.Write(body2)
}

func NewKontextProxy(
	globalCtx *globctx.Context,
	conf *Conf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*KonTextProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, &conf.ProxyConf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create KonText proxy: %w", err)
	}
	return &KonTextProxy{
		Proxy: proxy,
		conf:  conf,
	}, nil
}
