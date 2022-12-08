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
		services.LogEvent(ServiceName, t0, &userID, "dispatched request to 'treq'")
	}()
	if !strings.HasPrefix(req.URL.Path, ServicePath) {
		http.Error(w, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	uiStatus, userID, err := kp.analyzer.UserInducedResponseStatus(req)
	if err != nil {
		// TODO
		http.Error(
			w,
			fmt.Sprintf("Failed to proxy request: %s", err),
			uiStatus,
		)
		return

	} else if uiStatus > 400 && uiStatus < 500 {
		http.Error(w, http.StatusText(uiStatus), uiStatus)
		return
	}
	services.RestrictResponseTime(w, req, kp.readTimeoutSecs, kp.analyzer)
	passedHeaders := req.Header
	if kp.conf.UseHeaderXApiKey {
		passedHeaders["X-Api-Key"] = []string{services.GetSessionKey(req, kp.conf.SessionCookieName)}
	}

	serviceResp, err := kp.makeRequest(req)
	if err != nil {
		log.Error().Err(err).Msgf("failed to proxy request %s", req.URL.Path)
		http.Error(
			w,
			fmt.Sprintf("failed to proxy request: %s", err),
			http.StatusInternalServerError,
		)
		return
	}

	for k, v := range serviceResp.Headers {
		w.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	w.WriteHeader(serviceResp.StatusCode)
	w.Write(serviceResp.Body)
}

func (tp *TreqProxy) makeRequest(req *http.Request) (*services.ProxiedResponse, error) {
	body, header, err := tp.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]
		urlArgs := req.URL.Query()
		if _, ok := urlArgs["format"]; !ok {
			urlArgs["format"] = []string{"json"}
		}
		serviceResp := tp.apiProxy.Request(
			// TODO use some path builder here
			fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
			req.Method,
			req.Header,
			req.Body,
		)
		if serviceResp.Err != nil {
			return nil, serviceResp.Err
		}
		err = tp.cache.Set(req, string(serviceResp.Body), &serviceResp.Headers)
		if err != nil {
			return nil, err
		}
		return serviceResp, nil

	} else if err != nil {
		return nil, err
	}
	return &services.ProxiedResponse{Body: []byte(body), Headers: *header, StatusCode: 200, Err: nil}, nil
}

func NewTreqProxy(
	globalCtx *services.GlobalContext,
	conf *Conf,
	analyzer services.ReqAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	reqCounter chan<- alarms.RequestInfo,
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
	}
}
