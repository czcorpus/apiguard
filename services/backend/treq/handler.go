// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/alarms"
	"apiguard/cncdb/analyzer"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/backend"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
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
	cache           services.Cache
	analyzer        *analyzer.CNCUserAnalyzer
	cncDB           *sql.DB
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

func (tp *TreqProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
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
			req, ServiceName, time.Since(t0), cached, *loggedUserID, *indirect)
	}(&userID, &humanID, &indirectAPICall)
	if !strings.HasPrefix(req.URL.Path, ServicePath) {
		http.Error(w, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := tp.analyzer.UserInducedResponseStatus(req, ServiceName)
	userID = reqProps.UserID
	if reqProps.Error != nil {
		// TODO
		http.Error(
			w,
			fmt.Sprintf("Failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedStatus,
		)
		return

	} else if reqProps.ForbidsAccess() {
		http.Error(w, http.StatusText(reqProps.ProposedStatus), reqProps.ProposedStatus)
		return
	}
	services.RestrictResponseTime(w, req, tp.readTimeoutSecs, tp.analyzer)

	passedHeaders := req.Header
	if tp.cncAuthCookie != tp.conf.ExternalSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.analyzer.UserInternalCookieStatus(req, ServiceName)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
		passedHeaders[backend.HeaderAPIUserID] = []string{humanID.String()}

	} else {
		passedHeaders[backend.HeaderAPIUserID] = []string{userID.String()}
	}

	// first, remap cookie names
	if tp.reqUsesMappedSession(req) {
		err := backend.MapSessionCookie(
			req,
			tp.conf.ExternalSessionCookieName,
			tp.cncAuthCookie,
		)
		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}
	// then update auth cookie by x-api-key (if applicable)
	xApiKey := req.Header.Get(backend.HeaderAPIKey)
	if xApiKey != "" {
		cookie, err := req.Cookie(tp.cncAuthCookie)
		if err == nil {
			cookie.Value = xApiKey
		}
	}

	rt0 := time.Now().In(tp.globalCtx.TimezoneLocation)
	serviceResp := tp.makeRequest(req)
	tp.reporting <- services.ProxyProcReport{
		ProcTime: float32(time.Since(rt0).Seconds()),
		Status:   serviceResp.GetStatusCode(),
		Service:  ServiceName,
	}
	cached = serviceResp.IsCached()
	if serviceResp.GetError() != nil {
		log.Error().Err(serviceResp.GetError()).Msgf("failed to proxy request %s", req.URL.Path)
		http.Error(
			w,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.GetError()),
			http.StatusInternalServerError,
		)
		return
	}

	for k, v := range serviceResp.GetHeaders() {
		w.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	w.WriteHeader(serviceResp.GetStatusCode())
	w.Write(serviceResp.GetBody())
}

func (tp *TreqProxy) makeRequest(req *http.Request) services.BackendResponse {
	cacheApplCookies := []string{tp.conf.ExternalSessionCookieName, tp.cncAuthCookie}
	resp, err := tp.cache.Get(req, cacheApplCookies)
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
		err := tp.cache.Set(req, resp, cacheApplCookies)
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
	analyzer *analyzer.CNCUserAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	reqCounter chan<- alarms.RequestInfo,
	cache services.Cache,
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
		cncDB:           cncDB,
		apiProxy:        *services.NewAPIProxy(conf.GetProxyConf()),
		reqCounter:      reqCounter,
		cache:           cache,
		reporting:       reporting,
	}
}
