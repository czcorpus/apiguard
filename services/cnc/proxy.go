// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cnc

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/session"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type CoreProxy struct {
	globalCtx    *globctx.Context
	conf         *ProxyConf
	rConf        *EnvironConf
	guard        guard.ServiceGuard
	apiProxy     *proxy.APIProxy
	tDBWriter    reporting.ReportingWriter
	frontendHost string

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- guard.RequestInfo

	sessionValFactory func() session.HTTPSession
}

func (kp *CoreProxy) GlobalCtx() *globctx.Context {
	return kp.globalCtx
}

func (kp *CoreProxy) Guard() guard.ServiceGuard {
	return kp.guard
}

func (kp *CoreProxy) EnvironConf() *EnvironConf {
	return kp.rConf
}

// Preflight is used by APIGuard client (e.g. WaG) to find out whether
// the user using the client is logged in or not.
// To be able to recognize users logged in via CNC cookie (which is the
// one e.g. WaG does not use intentionally) we must actually make two
// tests - 1. frontend cookie, 2. backend cookie
func (kp *CoreProxy) Preflight(ctx *gin.Context) {
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	userId := common.InvalidUserID

	defer func(currUserID *common.UserID) {
		kp.globalCtx.BackendLogger.Log(
			ctx.Request,
			kp.rConf.ServiceKey,
			time.Since(t0),
			false,
			*currUserID,
			true,
			reporting.BackendActionTypePreflight,
		)
	}(&userId)

	reqProps := kp.guard.EvaluateRequest(ctx.Request)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user preflight request")
	userId = reqProps.ClientID
	if reqProps.Error != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to process preflight request: %s", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ProposedResponse >= 400 && reqProps.ProposedResponse < 500 {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedResponse), reqProps.ProposedResponse)
		return
	}
	ctx.Writer.WriteHeader(http.StatusNoContent)
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{})
}

// AnyPath is the main handler for KonText API actions.
func (kp *CoreProxy) AnyPath(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)

	defer kp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	if !strings.HasPrefix(ctx.Request.URL.Path, kp.rConf.ServicePath) {
		log.Error().Msgf("failed to proxy request - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := kp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err := kp.guard.DetermineTrueUserID(ctx.Request)
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

	if err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, kp.rConf.ReadTimeoutSecs, kp.guard, clientID); err != nil {
		return
	}

	if err := kp.ProcessReqHeaders(
		ctx, humanID, userID, &indirectAPICall,
	); err != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rt0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	serviceResp := kp.MakeRequest(ctx.Request, reqProps)
	kp.tDBWriter.Write(&reporting.ProxyProcReport{
		DateTime: time.Now().In(kp.globalCtx.TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.GetStatusCode(),
		Service:  kp.rConf.ServiceKey,
	})
	cached = serviceResp.IsCached()
	if serviceResp.GetError() != nil {
		log.Error().Err(serviceResp.GetError()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.GetError()),
			http.StatusInternalServerError,
		)
		return
	}

	for k, v := range serviceResp.GetHeaders() {
		ctx.Writer.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	ctx.Writer.WriteHeader(serviceResp.GetStatusCode())
	ctx.Writer.Write([]byte(serviceResp.GetBody()))
}

func (kp *CoreProxy) debugLogResponse(req *http.Request, res proxy.BackendResponse, err error) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	evt.Err(err)
	for hk, hv := range res.GetHeaders() {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("received proxied response")
}

func (kp *CoreProxy) debugLogRequest(req *http.Request) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	for hk, hv := range req.Header {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("about to proxy received request")
}

func NewCoreProxy(
	globalCtx *globctx.Context,
	conf *ProxyConf,
	gConf *EnvironConf,
	grd guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*CoreProxy, error) {
	proxy, err := proxy.NewAPIProxy(conf.GetCoreConf())
	if err != nil {
		return nil, fmt.Errorf("failed to create CoreProxy: %w", err)
	}
	fu, err := url.Parse(conf.FrontendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create CoreProxy: %w", err)
	}
	return &CoreProxy{
		globalCtx:         globalCtx,
		conf:              conf,
		rConf:             gConf,
		frontendHost:      fu.Host,
		guard:             grd,
		apiProxy:          proxy,
		reqCounter:        reqCounter,
		tDBWriter:         globalCtx.ReportingWriter,
		sessionValFactory: guard.CreateSessionValFactory(conf.SessionValType),
	}, nil
}
