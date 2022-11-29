// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/services"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

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
	apiProxy        services.APIProxy
}

func (kp *KontextProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if !strings.HasPrefix(path, ServicePath) {
		http.Error(w, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	/*
		uiStatus, err := kp.analyzer.UserInducedResponseStatus(req)
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
	*/
	path = path[len(ServicePath):]
	serviceResp := kp.apiProxy.Request(
		// TODO use some path builder here
		fmt.Sprintf("/%s?%s", path, req.URL.Query().Encode()),
		req.Method,
		req.Header,
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
) *KontextProxy {
	return &KontextProxy{
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
		cncDB:           cncDB,
		apiProxy: services.APIProxy{
			InternalURL: conf.InternalURL,
			ExternalURL: conf.ExternalURL,
		},
	}
}
