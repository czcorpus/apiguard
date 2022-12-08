// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
	"apiguard/cncdb/analyzer"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/defaults"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	ServicePath = "/service/kontext"
	ServiceName = "kontext"
)

type KontextProxy struct {
	globalCtx       *services.GlobalContext
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	defaults        map[string]defaults.Args
	analyzer        *analyzer.CNCUserAnalyzer
	cncDB           *sql.DB
	apiProxy        services.APIProxy

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- alarms.RequestInfo
}

func (kp *KontextProxy) SetDefault(req *http.Request, key, value string) error {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return errors.New("session not found")
	}
	kp.defaults[sessionID].Set(key, value)
	return nil
}

func (kp *KontextProxy) GetDefault(req *http.Request, key string) (string, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return "", errors.New("session not found")
	}
	return kp.defaults[sessionID].Get(key), nil
}

func (kp *KontextProxy) GetDefaults(req *http.Request) (defaults.Args, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return map[string][]string{}, errors.New("session not found")
	}
	return kp.defaults[sessionID], nil
}

func (kp *KontextProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
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
		services.LogEvent(ServiceName, t0, &userID, "dispatched request to 'kontext'")
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

	serviceResp := kp.makeRequest(req, reqProps)
	if serviceResp.Err != nil {
		log.Error().Err(serviceResp.Err).Msgf("failed to proxy request %s", req.URL.Path)
		http.Error(
			w,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.Err),
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

func (kp *KontextProxy) makeRequest(
	req *http.Request,
	reqProps services.ReqProperties,
) *services.ProxiedResponse {

	body, header, err := kp.cache.Get(req)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]

		dfltArgs, ok := kp.defaults[reqProps.SessionID]
		if !ok {
			dfltArgs = defaults.NewServiceDefaults("format", "corpname", "usesubcorp")
		}
		urlArgs := req.URL.Query()
		dfltArgs.Apply(urlArgs)
		serviceResp := kp.apiProxy.Request(
			// TODO use some path builder here
			fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
			req.Method,
			req.Header,
			req.Body,
		)
		if serviceResp.Err != nil {
			return serviceResp
		}
		serviceResp.Err = kp.cache.Set(req, string(serviceResp.Body), &serviceResp.Headers)
		return serviceResp

	} else if err != nil {
		return &services.ProxiedResponse{Err: err}
	}
	return &services.ProxiedResponse{
		Body:       []byte(body),
		Headers:    *header,
		StatusCode: 200,
	}
}

func NewKontextProxy(
	globalCtx *services.GlobalContext,
	conf *Conf,
	analyzer *analyzer.CNCUserAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	reqCounter chan<- alarms.RequestInfo,
	cache services.Cache,
) *KontextProxy {
	return &KontextProxy{
		globalCtx:       globalCtx,
		conf:            conf,
		analyzer:        analyzer,
		defaults:        make(map[string]defaults.Args),
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
