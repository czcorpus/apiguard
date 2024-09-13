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
	"apiguard/guard/sessionmap"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/reqcache"
	"apiguard/services/backend"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	ServicePath = "/service/treq"
	ServiceName = "treq"
)

type TreqProxy struct {
	globalCtx       *globctx.Context
	conf            *Conf
	cncAuthCookie   string
	readTimeoutSecs int
	guard           *sessionmap.Guard
	apiProxy        *proxy.APIProxy
	tDBWriter       reporting.ReportingWriter

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- guard.RequestInfo
}

func (tp *TreqProxy) reqUsesMappedSession(req *http.Request) bool {
	if tp.conf.ExternalSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(tp.conf.ExternalSessionCookieName)
	return err == nil
}

func (tp *TreqProxy) AnyPath(ctx *gin.Context) {
	var cached, indirectAPICall bool
	var userID, humanID common.UserID
	t0 := time.Now().In(tp.globalCtx.TimezoneLocation)
	defer func(currUserID, currHumanID *common.UserID, indirect *bool, created time.Time) {
		loggedUserID := currUserID
		if currHumanID.IsValid() && *currHumanID != tp.guard.AnonymousUserID {
			loggedUserID = currHumanID
		}
		if tp.reqCounter != nil {
			tp.reqCounter <- guard.RequestInfo{
				Created:     created,
				Service:     ServiceName,
				NumRequests: 1,
				UserID:      *loggedUserID,
			}
		}
		tp.globalCtx.BackendLogger.Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			reporting.BackendActionTypeQuery,
		)
	}(&userID, &humanID, &indirectAPICall, t0)
	if !strings.HasPrefix(ctx.Request.URL.Path, ServicePath) {
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := tp.guard.ClientInducedRespStatus(ctx.Request, ServiceName)
	userID = reqProps.UserID
	if reqProps.Error != nil {
		// TODO
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedStatus,
		)
		return

	} else if reqProps.ForbidsAccess() {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedStatus), reqProps.ProposedStatus)
		return
	}

	clientID := common.ClientID{
		IP:     ctx.RemoteIP(),
		UserID: userID,
	}
	guard.RestrictResponseTime(ctx.Writer, ctx.Request, tp.readTimeoutSecs, tp.guard, clientID)

	passedHeaders := ctx.Request.Header
	if tp.cncAuthCookie != tp.conf.ExternalSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.guard.UserInternalCookieStatus(ctx.Request, ServiceName)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
		passedHeaders[backend.HeaderAPIUserID] = []string{humanID.String()}

	} else {
		passedHeaders[backend.HeaderAPIUserID] = []string{userID.String()}
	}

	// first, remap cookie names
	if tp.reqUsesMappedSession(ctx.Request) {
		err := backend.MapSessionCookie(
			ctx.Request,
			tp.conf.ExternalSessionCookieName,
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
	tp.tDBWriter.Write(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.globalCtx.TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.GetStatusCode(),
		Service:  ServiceName,
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
	ctx.Writer.Write(serviceResp.GetBody())
}

func (tp *TreqProxy) makeRequest(req *http.Request) proxy.BackendResponse {
	cacheApplCookies := []string{tp.conf.ExternalSessionCookieName, tp.cncAuthCookie}
	resp, err := tp.globalCtx.Cache.Get(req, cacheApplCookies)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]
		resp = tp.apiProxy.Request(
			path,
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		err := tp.globalCtx.Cache.Set(req, resp, cacheApplCookies)
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
	cncAuthCookie string,
	guard *sessionmap.Guard,
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
		tDBWriter:       globalCtx.TimescaleDBWriter,
	}, nil
}
