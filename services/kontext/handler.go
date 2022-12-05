// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
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

func GetSessionKey(req *http.Request, cookieName string) string {
	var cookieValue string
	for _, cookie := range req.Cookies() {
		if cookie.Name == cookieName {
			cookieValue = cookie.Value
			break
		}
	}
	return cookieValue
}

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
		passedHeaders["X-Api-Key"] = []string{GetSessionKey(req, kp.conf.SessionCookieName)}
	}
	path = path[len(ServicePath):]
	urlArgs := req.URL.Query()
	if _, ok := urlArgs["format"]; !ok {
		urlArgs["format"] = []string{"json"}
	}
	serviceResp := kp.apiProxy.Request(
		// TODO use some path builder here
		fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
		req.Method,
		req.Header,
		req.Body,
	)
	if serviceResp.Err != nil {
		log.Error().Err(serviceResp.Err).Msgf("failed to proxy request %s", req.URL.Path)
		http.Error(
			w,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.Err),
			http.StatusInternalServerError,
		)
	}
	for k, v := range serviceResp.Headers {
		w.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	w.WriteHeader(serviceResp.StatusCode)
	w.Write(serviceResp.Body)
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
