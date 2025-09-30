// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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

package cnc

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/apiguard-common/cache"
	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-common/guard"
	"github.com/czcorpus/apiguard-common/proxy"
	"github.com/czcorpus/apiguard-common/reporting"
	"github.com/czcorpus/apiguard/session"

	guardImpl "github.com/czcorpus/apiguard/guard"
	proxyImpl "github.com/czcorpus/apiguard/proxy"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Proxy struct {
	globalCtx    *globctx.Context
	BackendURL   *url.URL
	conf         *ProxyConf
	rConf        *EnvironConf
	guard        guard.ServiceGuard
	apiProxy     *proxyImpl.CoreProxy
	tDBWriter    reporting.ReportingWriter
	frontendHost string

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- guardImpl.RequestInfo

	sessionValFactory func() session.HTTPSession

	userFinder guardImpl.UserFinder
}

func (kp *Proxy) FromCache(req *http.Request, opts ...func(*cache.CacheEntryOptions)) proxy.ResponseProcessor {
	data, err := kp.globalCtx.Cache.Get(req, opts...)
	if err == proxyImpl.ErrCacheMiss {
		return proxyImpl.NewThroughCacheResponse(req, kp.GlobalCtx().Cache, nil)

	} else if err != nil {
		return proxyImpl.NewThroughCacheResponse(req, kp.GlobalCtx().Cache, err)
	}

	return proxyImpl.NewCachedResponse(data.Status, data.Headers, data.Data)
}

func (kp *Proxy) ToCache(req *http.Request, data cache.CacheEntry, opts ...func(*cache.CacheEntryOptions)) error {
	return kp.globalCtx.Cache.Set(
		req,
		data,
		opts...,
	)
}

func (kp *Proxy) DeleteCookie(req *http.Request, name string) {
	cookies := req.Cookies()
	var cookieStrings []string

	for _, cookie := range cookies {
		if cookie.Name != name {
			cookieStrings = append(cookieStrings, cookie.String())
		}
	}

	if len(cookieStrings) > 0 {
		req.Header.Set("Cookie", strings.Join(cookieStrings, "; "))

	} else {
		req.Header.Del("Cookie")
	}
}

func (kp *Proxy) GlobalCtx() *globctx.Context {
	return kp.globalCtx
}

func (kp *Proxy) Guard() guard.ServiceGuard {
	return kp.guard
}

func (kp *Proxy) EnvironConf() *EnvironConf {
	return kp.rConf
}

func (kp *Proxy) Conf() *ProxyConf {
	return kp.conf
}

func (kp *Proxy) CountRequest(ctx *gin.Context, created time.Time, serviceKey string, userID common.UserID) {
	if kp.reqCounter != nil {
		kp.reqCounter <- guardImpl.RequestInfo{
			Created:     created,
			Service:     serviceKey,
			NumRequests: 1,
			UserID:      userID,
			IP:          ctx.ClientIP(),
		}
	}
}

func (kp *Proxy) ProxyRequest(
	path string,
	args url.Values,
	method string,
	headers http.Header,
	rbody io.Reader,
) *proxyImpl.BackendProxiedResponse {
	return kp.apiProxy.Request(
		path,
		args,
		method,
		headers,
		rbody,
	)
}

// Preflight is used by APIGuard client (e.g. WaG) to find out whether
// the user using the client is logged in or not.
// To be able to recognize users logged in via CNC cookie (which is the
// one e.g. WaG does not use intentionally) we must actually make two
// tests - 1. frontend cookie, 2. backend cookie
func (kp *Proxy) Preflight(ctx *gin.Context) {
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	userId := common.InvalidUserID

	defer func(currUserID *common.UserID) {
		kp.globalCtx.BackendLoggers.Get(kp.EnvironConf().ServiceKey).Log(
			ctx.Request,
			kp.rConf.ServiceKey,
			time.Since(t0),
			false,
			*currUserID,
			true,
			reporting.BackendActionTypePreflight,
		)
	}(&userId)

	reqProps := kp.guard.EvaluateRequest(ctx.Request, nil)
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

func (kp *Proxy) WriteReport(report *reporting.ProxyProcReport) {
	kp.tDBWriter.Write(report)
}

// AnyPath is the main handler for KonText API actions.
func (kp *Proxy) AnyPath(ctx *gin.Context) {
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
	serviceResp := kp.HandleRequest(ctx.Request, reqProps, true)
	cached = serviceResp.IsCacheHit()
	kp.tDBWriter.Write(&reporting.ProxyProcReport{
		DateTime: time.Now().In(kp.globalCtx.TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.Response().GetStatusCode(),
		Service:  kp.rConf.ServiceKey,
		IsCached: cached,
	})
	if serviceResp.Error() != nil {
		log.Error().Err(serviceResp.Error()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)

		proxyImpl.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %s", serviceResp.Error()),
			http.StatusInternalServerError,
		)
		return
	}
	serviceResp.WriteResponse(ctx.Writer)
}

func (kp *Proxy) debugLogResponse(req *http.Request, res proxy.BackendResponse) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	evt.Err(res.Error())
	for hk, hv := range res.GetHeaders() {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("received proxied response")
}

func (kp *Proxy) debugLogRequest(req *http.Request) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	for hk, hv := range req.Header {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("about to proxy received request")
}

func NewProxy(
	globalCtx *globctx.Context,
	conf *ProxyConf,
	gConf *EnvironConf,
	grd guard.ServiceGuard,
	reqCounter chan<- guardImpl.RequestInfo,
) (*Proxy, error) {
	proxy, err := proxyImpl.NewCoreProxy(conf.GetCoreConf())
	if err != nil {
		return nil, fmt.Errorf("failed to create CoreProxy: %w", err)
	}
	fu, err := url.Parse(conf.FrontendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create CoreProxy: %w", err)
	}
	bu, err := url.Parse(conf.BackendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create CoreProxy: %w", err)
	}
	return &Proxy{
		globalCtx:         globalCtx,
		conf:              conf,
		rConf:             gConf,
		frontendHost:      fu.Host,
		BackendURL:        bu,
		guard:             grd,
		apiProxy:          proxy,
		reqCounter:        reqCounter,
		tDBWriter:         globalCtx.ReportingWriter,
		sessionValFactory: guardImpl.CreateSessionValFactory(conf.SessionValType),
		userFinder: 	   guardImpl.NewUserFinder(globalCtx),
	}, nil
}
