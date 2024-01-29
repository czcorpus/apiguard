// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/alarms"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/monitoring"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/backend"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	ServicePath = "/service/treq"
	ServiceName = "treq"
)

type TreqProxy struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	cncAuthCookie   string
	readTimeoutSecs int
	analyzer        *guard.CNCUserAnalyzer
	apiProxy        services.APIProxy
	reporting       chan<- services.ProxyProcReport

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- alarms.RequestInfo
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
	defer func(currUserID, currHumanID *common.UserID, indirect *bool) {
		if tp.reqCounter != nil {
			tp.reqCounter <- alarms.RequestInfo{
				Service:     ServiceName,
				NumRequests: 1,
				UserID:      *currUserID,
			}
		}
		loggedUserID := currUserID
		if currHumanID.IsValid() && *currHumanID != tp.analyzer.AnonymousUserID {
			loggedUserID = currHumanID
		}
		tp.globalCtx.BackendLogger.Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			monitoring.BackendActionTypeQuery,
		)
	}(&userID, &humanID, &indirectAPICall)
	if !strings.HasPrefix(ctx.Request.URL.Path, ServicePath) {
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := tp.analyzer.UserInducedResponseStatus(ctx.Request, ServiceName)
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
	services.RestrictResponseTime(ctx.Writer, ctx.Request, tp.readTimeoutSecs, tp.analyzer)

	passedHeaders := ctx.Request.Header
	if tp.cncAuthCookie != tp.conf.ExternalSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.analyzer.UserInternalCookieStatus(ctx.Request, ServiceName)
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
	tp.reporting <- services.ProxyProcReport{
		ProcTime: float32(time.Since(rt0).Seconds()),
		Status:   serviceResp.GetStatusCode(),
		Service:  ServiceName,
	}
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

func (tp *TreqProxy) makeRequest(req *http.Request) services.BackendResponse {
	cacheApplCookies := []string{tp.conf.ExternalSessionCookieName, tp.cncAuthCookie}
	resp, err := tp.globalCtx.Cache.Get(req, cacheApplCookies)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]
		urlArgs := req.URL.Query()
		resp = tp.apiProxy.Request(
			// TODO use some path builder here
			fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
			req.Method,
			req.Header,
			req.Body,
		)
		err := tp.globalCtx.Cache.Set(req, resp, cacheApplCookies)
		if err != nil {
			return &services.ProxiedResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &services.ProxiedResponse{Err: err}
	}
	return resp
}

func NewTreqProxy(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	cncAuthCookie string,
	analyzer *guard.CNCUserAnalyzer,
	readTimeoutSecs int,
	reqCounter chan<- alarms.RequestInfo,
) *TreqProxy {
	reporting := make(chan services.ProxyProcReport)
	go func() {
		influx.RunWriteConsumerSync(globalCtx.InfluxDB, "proxy", reporting)
	}()
	return &TreqProxy{
		globalCtx:       globalCtx,
		conf:            conf,
		cncAuthCookie:   cncAuthCookie,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
		apiProxy:        *services.NewAPIProxy(conf.GetCoreConf()),
		reqCounter:      reqCounter,
		reporting:       reporting,
	}
}
