// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
	"apiguard/cncdb/analyzer"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/backend"
	"apiguard/services/defaults"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/influx"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

const (
	AuthTokenEntry    = "personal_access_token"
	ServicePath       = "/service/kontext"
	ServiceName       = "kontext"
	CNCPortalLoginURL = "https://www.korpus.cz/login"
)

type KontextProxy struct {
	globalCtx       *ctx.GlobalContext
	conf            *Conf
	cncAuthCookie   string
	readTimeoutSecs int
	cache           services.Cache
	defaults        *collections.ConcurrentMap[string, defaults.Args]
	analyzer        *analyzer.CNCUserAnalyzer
	cncDB           *sql.DB
	apiProxy        *services.APIProxy
	reporting       chan<- services.ProxyProcReport

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
	kp.defaults.Get(sessionID).Set(key, value)
	return nil
}

func (kp *KontextProxy) GetDefault(req *http.Request, key string) (string, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return "", errors.New("session not found")
	}
	return kp.defaults.Get(sessionID).Get(key), nil
}

func (kp *KontextProxy) GetDefaults(req *http.Request) (defaults.Args, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return map[string][]string{}, errors.New("session not found")
	}
	return kp.defaults.Get(sessionID), nil
}

// Preflight is used by APIGuard client (e.g. WaG) to find out whether
// the user using the client is logged in or not.
// To be able to recognize users logged in via CNC cookie (which is the
// one e.g. WaG does not use intentionally) we must actually make two
// tests - 1. external cookie, 2. internal cookie
func (kp *KontextProxy) Preflight(w http.ResponseWriter, req *http.Request) {
	reqProps := kp.analyzer.UserInducedResponseStatus(req, ServiceName)
	if reqProps.Error != nil {
		http.Error(
			w,
			fmt.Sprintf("Failed to process preflight request: %s", reqProps.Error),
			reqProps.ProposedStatus,
		)
		return

	} else if reqProps.ProposedStatus >= 400 && reqProps.ProposedStatus < 500 {
		http.Error(w, http.StatusText(reqProps.ProposedStatus), reqProps.ProposedStatus)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	uniresp.WriteJSONResponse(w, map[string]any{})
}

func (kp *KontextProxy) reqUsesMappedSession(req *http.Request) bool {
	if kp.conf.ExternalSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(kp.conf.ExternalSessionCookieName)
	return err == nil
}

// Login is a custom Proxy for CNC portals' central login action.
// We use it in situations where we need a "hidden" API login using
// a special user account for unauthorized users. In such case,
// the CNC login will still return standard cookies which would
// make the hidden user visible. So our proxy action will rename
// a respective cookie and will allow a custom web application (e.g. WaG)
// to use this special cookie.
func (kp *KontextProxy) Login(w http.ResponseWriter, req *http.Request) {
	postData := url.Values{}
	postData.Set(AuthTokenEntry, req.FormValue(AuthTokenEntry))
	req2, err := http.NewRequest(
		http.MethodPost,
		CNCPortalLoginURL,
		strings.NewReader(postData.Encode()),
	)
	if err != nil {
		log.Error().Err(err).Msgf("failed to perform login")
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // only for internal network communication
		},
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req2)
	if err != nil {
		log.Error().Err(err).Msgf("failed to perform login")
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionErrorFrom(err), resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msgf("failed to perform login")
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	respMsg := make([]string, 0, 1)
	err = json.Unmarshal(body, &respMsg)
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionErrorFrom(err), resp.StatusCode)
		return
	}

	if respMsg[0] == "Invalid credentials" {
		log.Error().Err(err).Msgf("failed to perform login")
		uniresp.WriteCustomJSONErrorResponse(w, respMsg, http.StatusUnauthorized)
		return
	}

	cookies := resp.Cookies()
	for _, cookie := range cookies {
		cCopy := *cookie
		if cCopy.Name == kp.cncAuthCookie && kp.conf.ExternalSessionCookieName != "" {
			cCopy.Name = kp.conf.ExternalSessionCookieName
			log.Debug().
				Str("internalCookie", kp.cncAuthCookie).
				Str("externalCookie", kp.conf.ExternalSessionCookieName).
				Str("value", cCopy.Value).
				Msg("login action - mapping back internal cookie")
		}
		http.SetCookie(w, &cCopy)
	}
	uniresp.WriteJSONResponse(w, respMsg)
}

// AnyPath is the main handler for KonText API actions.
func (kp *KontextProxy) AnyPath(w http.ResponseWriter, req *http.Request) {
	var userID, humanID common.UserID
	var cached bool
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	defer func(currUserID *common.UserID, currHumanID *common.UserID) {
		if kp.reqCounter != nil {
			kp.reqCounter <- alarms.RequestInfo{
				Service:     ServiceName,
				NumRequests: 1,
				UserID:      *currUserID,
			}
		}
		loggedUserID := currUserID
		if currHumanID.IsValid() && *currHumanID != kp.analyzer.AnonymousUserID {
			loggedUserID = currHumanID
		}
		kp.globalCtx.BackendLogger.Log(ServiceName, time.Since(t0), &cached, loggedUserID)
	}(&userID, &humanID)
	if !strings.HasPrefix(req.URL.Path, ServicePath) {
		log.Error().Msgf("failed to proxy request - invalid path detected")
		http.Error(w, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := kp.analyzer.UserInducedResponseStatus(req, ServiceName)
	userID = reqProps.UserID
	if reqProps.Error != nil {
		// TODO
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request")
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
	services.RestrictResponseTime(w, req, kp.readTimeoutSecs, kp.analyzer)

	passedHeaders := req.Header

	if kp.cncAuthCookie != kp.conf.ExternalSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = kp.analyzer.UserInternalCookieStatus(req, ServiceName)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
		passedHeaders[backend.HeaderAPIUserID] = []string{humanID.String()}

	} else {
		passedHeaders[backend.HeaderAPIUserID] = []string{userID.String()}
	}

	if kp.conf.UseHeaderXApiKey {
		if kp.reqUsesMappedSession(req) {
			passedHeaders[backend.HeaderAPIKey] = []string{services.GetCookieValue(req, kp.conf.ExternalSessionCookieName)}

		} else {
			passedHeaders[backend.HeaderAPIKey] = []string{services.GetCookieValue(req, kp.cncAuthCookie)}
		}

	} else if kp.reqUsesMappedSession(req) {

		err := backend.MapSessionCookie(
			req,
			kp.conf.ExternalSessionCookieName,
			kp.cncAuthCookie,
		)
		if err != nil {
			log.Error().Err(reqProps.Error).Msgf("failed to proxy request - cookie mapping")
			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	rt0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	serviceResp := kp.makeRequest(req, reqProps)
	kp.reporting <- services.ProxyProcReport{
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
	w.Write([]byte(serviceResp.GetBody()))
}

func (kp *KontextProxy) makeRequest(
	req *http.Request,
	reqProps services.ReqProperties,
) services.BackendResponse {
	cacheApplCookies := []string{kp.cncAuthCookie, kp.conf.ExternalSessionCookieName}
	resp, err := kp.cache.Get(req, cacheApplCookies)
	if err == reqcache.ErrCacheMiss {
		path := req.URL.Path[len(ServicePath):]
		dfltArgs, ok := kp.defaults.GetWithTest(reqProps.SessionID)
		if !ok {
			dfltArgs = defaults.NewServiceDefaults("format", "corpname", "usesubcorp")
			dfltArgs.Set("format", "json")
			kp.defaults.Set(reqProps.SessionID, dfltArgs)
		}
		urlArgs := req.URL.Query()
		dfltArgs.ApplyTo(urlArgs)
		resp = kp.apiProxy.Request(
			// TODO use some path builder here
			fmt.Sprintf("/%s?%s", path, urlArgs.Encode()),
			req.Method,
			req.Header,
			req.Body,
		)
		err = kp.cache.Set(req, resp, cacheApplCookies)
		if err != nil {
			resp = &services.ProxiedResponse{Err: err}
		}
		return resp
	}
	if err != nil {
		return &services.ProxiedResponse{Err: err}
	}
	return resp
}

func NewKontextProxy(
	globalCtx *ctx.GlobalContext,
	conf *Conf,
	cncAuthCookie string,
	analyzer *analyzer.CNCUserAnalyzer,
	readTimeoutSecs int,
	cncDB *sql.DB,
	reqCounter chan<- alarms.RequestInfo,
	cache services.Cache,
) *KontextProxy {
	reporting := make(chan services.ProxyProcReport)
	go func() {
		influx.RunWriteConsumerSync(globalCtx.InfluxDB, "proxy", reporting)
	}()
	return &KontextProxy{
		globalCtx:       globalCtx,
		conf:            conf,
		cncAuthCookie:   cncAuthCookie,
		analyzer:        analyzer,
		defaults:        collections.NewConcurrentMap[string, defaults.Args](),
		readTimeoutSecs: readTimeoutSecs,
		cncDB:           cncDB,
		apiProxy:        services.NewAPIProxy(conf.GetProxyConf()),
		reqCounter:      reqCounter,
		cache:           cache,
		reporting:       reporting,
	}
}
