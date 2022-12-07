// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

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
	ServicePath = "/service/kontext"
)

type KontextProxy struct {
	conf            *Conf
	readTimeoutSecs int
	cache           services.Cache
	analyzer        services.ReqAnalyzer
	cncDB           *sql.DB
	location        *time.Location
	apiProxy        services.APIProxy

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- alarms.RequestInfo
}

func (kp *KontextProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
	var userID int
	t0 := time.Now().In(kp.location)
	defer func() {
		if kp.reqCounter != nil {
			kp.reqCounter <- alarms.RequestInfo{
				Service:     "kontext",
				NumRequests: 1,
				UserID:      userID,
			}
		}
		t1 := time.Since(t0)
		log.Debug().
			Float64("procTime", t1.Seconds()).
			Msgf("dispatched request to 'kontext'")
	}()
	path := req.URL.Path
	if !strings.HasPrefix(path, ServicePath) {
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
	path = path[len(ServicePath):]
	urlArgs := req.URL.Query()
	if _, ok := urlArgs["format"]; !ok {
		urlArgs["format"] = []string{"json"}
	}
	// TODO use some path builder here
	url := fmt.Sprintf("/%s?%s", path, urlArgs.Encode())
	serviceResp, err := kp.makeRequest(url, req)
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

func (kp *KontextProxy) makeRequest(url string, req *http.Request) (*services.ProxiedResponse, error) {
	body, header, err := kp.cache.Get(url)
	if err == reqcache.ErrCacheMiss {
		serviceResp := kp.apiProxy.Request(
			url,
			req.Method,
			req.Header,
			req.Body,
		)
		if serviceResp.Err != nil {
			return nil, serviceResp.Err
		}
		err = kp.cache.Set(url, string(serviceResp.Body), &serviceResp.Headers, req)
		if err != nil {
			return nil, err
		}
		return serviceResp, nil

	} else if err != nil {
		return nil, err
	}
	return &services.ProxiedResponse{Body: []byte(body), Headers: *header, StatusCode: 200, Err: nil}, nil
}

func NewKontextProxy(
	conf *Conf,
	analyzer services.ReqAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	loc *time.Location,
	reqCounter chan<- alarms.RequestInfo,
) *KontextProxy {
	return &KontextProxy{
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
		cncDB:           cncDB,
		location:        loc,
		apiProxy: services.APIProxy{
			InternalURL: conf.InternalURL,
			ExternalURL: conf.ExternalURL,
		},
		reqCounter: reqCounter,
	}
}
