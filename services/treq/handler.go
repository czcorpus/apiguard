// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/alarms"
	"apiguard/reqcache"
	"apiguard/services"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	ServicePath = "/service/treq"
	ServiceName = "treq"
)

type TreqProxy struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        services.ReqAnalyzer
	cncDB           *sql.DB
	apiProxy        services.APIProxy

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- alarms.RequestInfo
}

func (kp *TreqProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
	var cached bool
	var userID int
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	defer func() {
		if kp.reqCounter != nil {
			kp.reqCounter <- alarms.RequestInfo{
				Service:     ServiceName,
				NumRequests: 1,
				UserID:      userID,
			}
		}
		services.LogServiceRequest(ServiceName, t0, &cached, &userID)
	}()
	if !strings.HasPrefix(req.URL.Path, ServicePath) {
		http.Error(w, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := kp.analyzer.UserInducedResponseStatus(req)
	if reqProps.Error != nil {
		// TODO
		http.Error(
			w,
			fmt.Sprintf("Failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedStatus,
		)
		return

	} else if reqProps.ProposedStatus > 400 && reqProps.ProposedStatus < 500 {
		http.Error(w, http.StatusText(reqProps.ProposedStatus), reqProps.ProposedStatus)
		return
	}
	services.RestrictResponseTime(w, req, kp.readTimeoutSecs, kp.analyzer)
	passedHeaders := req.Header
	if kp.conf.UseHeaderXApiKey {
		passedHeaders["X-Api-Key"] = []string{services.GetSessionKey(req, kp.conf.SessionCookieName)}
	}

	serviceResp := kp.makeRequest(req)
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
	resp, err := tp.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]
		urlArgs := req.URL.Query()
		if _, ok := urlArgs["format"]; !ok {
			urlArgs["format"] = []string{"json"}
		}
		resp = tp.apiProxy.Request(
			// TODO use some path builder here
			fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
			req.Method,
			req.Header,
			req.Body,
		)
		err := tp.cache.Set(req, resp)
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
	globalCtx *services.GlobalContext,
	conf *Conf,
	analyzer services.ReqAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	reqCounter chan<- alarms.RequestInfo,
	cache services.Cache,
) *TreqProxy {
	return &TreqProxy{
		globalCtx:       globalCtx,
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
		cncDB:           cncDB,
		apiProxy: services.APIProxy{
			InternalURL: conf.InternalURL,
			ExternalURL: conf.ExternalURL,
		},
		reqCounter: reqCounter,
		cache:      cache,
	}
}
