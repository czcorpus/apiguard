// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/cncauth"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/backend"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type TreqProxy struct {
	globalCtx       *globctx.Context
	conf            *Conf
	cncAuthCookie   string
	readTimeoutSecs int
	guard           *cncauth.Guard
	apiProxy        *proxy.APIProxy
	tDBWriter       reporting.ReportingWriter
	servicePath     string
	serviceKey      string

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- guard.RequestInfo
}

func (tp *TreqProxy) reqUsesMappedSession(req *http.Request) bool {
	if tp.conf.FrontendSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(tp.conf.FrontendSessionCookieName)
	return err == nil
}

func (tp *TreqProxy) AnyPath(ctx *gin.Context) {
	var cached, indirectAPICall bool
	var clientID, humanID common.UserID
	t0 := time.Now().In(tp.globalCtx.TimezoneLocation)
	defer func(currUserID, currHumanID *common.UserID, indirect *bool, created time.Time) {
		loggedUserID := currUserID
		if currHumanID.IsValid() && tp.guard.TestUserIsAnonymous(*currHumanID) {
			loggedUserID = currHumanID
		}
		if tp.reqCounter != nil {
			tp.reqCounter <- guard.RequestInfo{
				Created:     created,
				Service:     tp.serviceKey,
				NumRequests: 1,
				UserID:      *loggedUserID,
				IP:          ctx.ClientIP(),
			}
		}
		tp.globalCtx.BackendLogger.Log(
			ctx.Request,
			tp.serviceKey,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			reporting.BackendActionTypeQuery,
		)
	}(&clientID, &humanID, &indirectAPICall, t0)
	if !strings.HasPrefix(ctx.Request.URL.Path, tp.servicePath) {
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := tp.guard.EvaluateRequest(ctx.Request)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user treq/* request")
	clientID = reqProps.ClientID
	if reqProps.Error != nil {
		// TODO
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ForbidsAccess() {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedResponse), reqProps.ProposedResponse)
		return
	}

	passedHeaders := ctx.Request.Header
	if tp.cncAuthCookie != tp.conf.FrontendSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.guard.DetermineTrueUserID(ctx.Request)
		clientID = humanID
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
	}
	passedHeaders[backend.HeaderAPIUserID] = []string{clientID.String()}
	guard.RestrictResponseTime(
		ctx.Writer,
		ctx.Request,
		tp.readTimeoutSecs,
		tp.guard,
		common.ClientID{
			IP: ctx.RemoteIP(),
			ID: clientID,
		},
	)

	// first, remap cookie names
	if tp.reqUsesMappedSession(ctx.Request) {
		err := backend.MapFrontendCookieToBackend(
			ctx.Request,
			tp.conf.FrontendSessionCookieName,
			tp.cncAuthCookie,
		)
		if err != nil {
			http.Error(
				ctx.Writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}
	// then update auth cookie by x-api-key (if applicable)
	xApiKey := ctx.Request.Header.Get(backend.HeaderAPIKey)
	if xApiKey != "" {
		cookie, err := ctx.Request.Cookie(tp.cncAuthCookie)
		if err == nil {
			cookie.Value = xApiKey
		}
	}

	rt0 := time.Now().In(tp.globalCtx.TimezoneLocation)
	serviceResp := tp.makeRequest(ctx.Request)
	cached = serviceResp.IsCached()
	tp.tDBWriter.Write(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.globalCtx.TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.GetStatusCode(),
		Service:  tp.serviceKey,
		IsCached: cached,
	})
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
	ctx.Writer.Write(serviceResp.GetBody())
}

func (tp *TreqProxy) makeRequest(req *http.Request) proxy.BackendResponse {
	cacheApplCookies := []string{tp.conf.FrontendSessionCookieName, tp.cncAuthCookie}
	resp, err := tp.globalCtx.Cache.Get(req, proxy.CachingWithCookies(cacheApplCookies))
	if err == proxy.ErrCacheMiss {
		path := req.URL.Path[len(tp.servicePath):]
		resp = tp.apiProxy.Request(
			path,
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		err := tp.globalCtx.Cache.Set(req, resp, proxy.CachingWithCookies(cacheApplCookies))
		if err != nil {
			return &proxy.ProxiedResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &proxy.ProxiedResponse{Err: err}
	}
	return resp
}

func NewTreqProxy(
	globalCtx *globctx.Context,
	conf *Conf,
	sid int,
	cncAuthCookie string,
	guard *cncauth.Guard,
	readTimeoutSecs int,
	reqCounter chan<- guard.RequestInfo,
) (*TreqProxy, error) {
	proxy, err := proxy.NewAPIProxy(conf.GetCoreConf())
	if err != nil {
		return nil, err
	}
	return &TreqProxy{
		globalCtx:       globalCtx,
		conf:            conf,
		cncAuthCookie:   cncAuthCookie,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
		apiProxy:        proxy,
		reqCounter:      reqCounter,
		tDBWriter:       globalCtx.ReportingWriter,
		serviceKey:      fmt.Sprintf("%d/treq", sid),
		servicePath:     fmt.Sprintf("/service/%d/treq", sid),
	}, nil
}
