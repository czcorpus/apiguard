// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cnc

import (
	"apiguard/alarms"
	"apiguard/cnc/guard"
	"apiguard/cnc/session"
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/monitoring"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/backend"
	"apiguard/services/defaults"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type loginResponse struct {
	code    int
	message string
	cookies []*http.Cookie
	err     error
}

func (resp loginResponse) isInvalidCredentials() bool {
	return resp.message == "Invalid credentials"
}

type CoreProxy struct {
	globalCtx *ctx.GlobalContext
	conf      *ProxyConf
	rConf     *EnvironConf
	analyzer  *guard.CNCUserAnalyzer
	apiProxy  *services.APIProxy
	reporting chan<- services.ProxyProcReport

	// reqCounter can be used to send info about number of request
	// to an alarm service. Please note that this value can be nil
	// (in such case, nothing is sent)
	reqCounter chan<- alarms.RequestInfo
}

// Preflight is used by APIGuard client (e.g. WaG) to find out whether
// the user using the client is logged in or not.
// To be able to recognize users logged in via CNC cookie (which is the
// one e.g. WaG does not use intentionally) we must actually make two
// tests - 1. external cookie, 2. internal cookie
func (kp *CoreProxy) Preflight(ctx *gin.Context) {
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	userId := common.InvalidUserID

	defer func(currUserID *common.UserID) {
		kp.globalCtx.BackendLogger.Log(
			ctx.Request,
			kp.rConf.ServiceName,
			time.Since(t0),
			false,
			*currUserID,
			true,
			monitoring.BackendActionTypePreflight,
		)
	}(&userId)

	reqProps := kp.analyzer.UserInducedResponseStatus(ctx.Request, kp.rConf.ServiceName)
	userId = reqProps.UserID
	if reqProps.Error != nil {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to process preflight request: %s", reqProps.Error),
			reqProps.ProposedStatus,
		)
		return

	} else if reqProps.ProposedStatus >= 400 && reqProps.ProposedStatus < 500 {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedStatus), reqProps.ProposedStatus)
		return
	}
	ctx.Writer.WriteHeader(http.StatusNoContent)
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{})
}

func (kp *CoreProxy) reqUsesMappedSession(req *http.Request) bool {
	if kp.conf.ExternalSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(kp.conf.ExternalSessionCookieName)
	return err == nil
}

// loginFromCtx performs an HTTP request with
// CNC login based on current ctx (where we're
// interested mainly in user's request properties).
func (kp *CoreProxy) loginFromCtx(ctx *gin.Context) loginResponse {
	postData := url.Values{}
	postData.Set(kp.rConf.AuthTokenEntry, ctx.Request.FormValue(kp.rConf.AuthTokenEntry))
	req2, err := http.NewRequest(
		http.MethodPost,
		kp.rConf.CNCPortalLoginURL,
		strings.NewReader(postData.Encode()),
	)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
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
		return loginResponse{
			code: resp.StatusCode,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	defer resp.Body.Close()
	respMsg := make([]string, 0, 1)
	err = json.Unmarshal(body, &respMsg)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	return loginResponse{
		code:    http.StatusOK,
		message: respMsg[0],
		cookies: resp.Cookies(),
	}
}

// Login is a custom Proxy for CNC portals' central login action.
// We use it in situations where we need a "hidden" API login using
// a special user account for unauthorized users. Without additional
// action, such CNC login would still return standard cookies and made
// hidden user visible. So our proxy action will rename a respective
// cookie and will allow a custom web application (e.g. WaG)
// to use this special cookie.
func (kp *CoreProxy) Login(ctx *gin.Context) {
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	userId := common.InvalidUserID

	defer func(currUserID *common.UserID) {
		kp.globalCtx.BackendLogger.Log(
			ctx.Request,
			kp.rConf.ServiceName,
			time.Since(t0),
			false,
			*currUserID,
			true,
			monitoring.BackendActionTypeLogin,
		)
	}(&userId)

	resp := kp.loginFromCtx(ctx)
	if resp.err != nil {
		log.Error().Err(resp.err).Msgf("failed to perform login")
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(resp.err), resp.code)
		return
	}

	if resp.isInvalidCredentials() {
		log.Error().Err(fmt.Errorf("invalid credentials")).Msgf("failed to perform login")
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, resp.message, http.StatusUnauthorized)
		return
	}

	for _, cookie := range resp.cookies {
		cCopy := *cookie
		if cCopy.Name == kp.rConf.CNCAuthCookie && kp.conf.ExternalSessionCookieName != "" {
			var err error
			var sessionID session.CNCSessionValue
			sessionID.UpdateFrom(cCopy.Value)
			userId, err = guard.FindUserBySession(kp.globalCtx.CNCDB, sessionID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to obtain user ID after successful. Ignoring.")
			}
			cCopy.Name = kp.conf.ExternalSessionCookieName
			log.Debug().
				Str("internalCookie", kp.rConf.CNCAuthCookie).
				Str("externalCookie", kp.conf.ExternalSessionCookieName).
				Str("value", cCopy.Value).
				Msg("login action - mapping back internal cookie")
		}
		http.SetCookie(ctx.Writer, &cCopy)
	}
	uniresp.WriteJSONResponse(ctx.Writer, resp.message)
}

// AnyPath is the main handler for KonText API actions.
func (kp *CoreProxy) AnyPath(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)

	defer func(currUserID *common.UserID, currHumanID *common.UserID, indirect *bool) {
		if kp.reqCounter != nil {
			kp.reqCounter <- alarms.RequestInfo{
				Service:     kp.rConf.ServiceName,
				NumRequests: 1,
				UserID:      *currUserID,
			}
		}
		loggedUserID := currUserID
		if currHumanID.IsValid() && *currHumanID != kp.analyzer.AnonymousUserID {
			loggedUserID = currHumanID
		}
		kp.globalCtx.BackendLogger.Log(
			ctx.Request,
			kp.rConf.ServiceName,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			monitoring.BackendActionTypeQuery,
		)
	}(&userID, &humanID, &indirectAPICall)

	if !strings.HasPrefix(ctx.Request.URL.Path, kp.rConf.ServicePath) {
		log.Error().Msgf("failed to proxy request - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps := kp.analyzer.UserInducedResponseStatus(ctx.Request, kp.rConf.ServiceName)
	userID = reqProps.UserID
	if reqProps.Error != nil {
		// TODO
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request")
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
	if err := services.RestrictResponseTime(ctx.Writer, ctx.Request, kp.rConf.ReadTimeoutSecs, kp.analyzer); err != nil {
		return
	}

	passedHeaders := ctx.Request.Header

	if kp.rConf.CNCAuthCookie != kp.conf.ExternalSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = kp.analyzer.UserInternalCookieStatus(ctx.Request, kp.rConf.ServiceName)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
		passedHeaders[backend.HeaderAPIUserID] = []string{humanID.String()}

	} else {
		passedHeaders[backend.HeaderAPIUserID] = []string{userID.String()}
	}

	if passedHeaders.Get(backend.HeaderIndirectCall) != "" {
		indirectAPICall = true
	}

	if kp.conf.UseHeaderXApiKey {
		if kp.reqUsesMappedSession(ctx.Request) {
			passedHeaders[backend.HeaderAPIKey] = []string{
				services.GetCookieValue(ctx.Request, kp.conf.ExternalSessionCookieName),
			}

		} else {
			passedHeaders[backend.HeaderAPIKey] = []string{
				services.GetCookieValue(ctx.Request, kp.rConf.CNCAuthCookie),
			}
		}

	} else if kp.reqUsesMappedSession(ctx.Request) {

		err := backend.MapSessionCookie(
			ctx.Request,
			kp.conf.ExternalSessionCookieName,
			kp.rConf.CNCAuthCookie,
		)
		if err != nil {
			log.Error().Err(reqProps.Error).Msgf("failed to proxy request - cookie mapping")
			http.Error(
				ctx.Writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	rt0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	serviceResp := kp.makeRequest(ctx.Request, reqProps)
	kp.reporting <- services.ProxyProcReport{
		ProcTime: float32(time.Since(rt0).Seconds()),
		Status:   serviceResp.GetStatusCode(),
		Service:  kp.rConf.ServiceName,
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
	ctx.Writer.Write([]byte(serviceResp.GetBody()))
}

func (kp *CoreProxy) CreateDefaultArgs(reqProps services.ReqProperties) defaults.Args {
	return make(defaults.Args)
}

func (kp *CoreProxy) debugLogResponse(req *http.Request, res services.BackendResponse, err error) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	evt.Err(err)
	for hk, hv := range res.GetHeaders() {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("received proxied response")
}

func (kp *CoreProxy) debugLogRequest(req *http.Request) {
	evt := log.Debug()
	evt.Str("url", req.URL.String())
	for hk, hv := range req.Header {
		if len(hv) > 0 {
			evt.Str(hk, hv[0])
		}
	}
	evt.Msg("about to proxy received request")
}

func (kp *CoreProxy) makeRequest(
	req *http.Request,
	reqProps services.ReqProperties,
) services.BackendResponse {
	kp.debugLogRequest(req)
	cacheApplCookies := []string{kp.rConf.CNCAuthCookie, kp.conf.ExternalSessionCookieName}
	resp, err := kp.globalCtx.Cache.Get(req, cacheApplCookies)
	if err == reqcache.ErrCacheMiss {
		dfltArgs := kp.CreateDefaultArgs(reqProps)
		urlArgs := req.URL.Query()
		dfltArgs.ApplyTo(urlArgs)
		resp = kp.apiProxy.Request(
			// TODO use some path builder here
			path.Join("/", req.URL.Path[len(kp.rConf.ServicePath):])+"?"+urlArgs.Encode(),
			req.Method,
			req.Header,
			req.Body,
		)
		kp.debugLogResponse(req, resp, err)
		err = kp.globalCtx.Cache.Set(req, resp, cacheApplCookies)
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

func NewCoreProxy(
	globalCtx *ctx.GlobalContext,
	conf *ProxyConf,
	gConf *EnvironConf,
	analyzer *guard.CNCUserAnalyzer,
	reqCounter chan<- alarms.RequestInfo,
) *CoreProxy {
	reporting := make(chan services.ProxyProcReport)
	go func() {
		influx.RunWriteConsumerSync(globalCtx.InfluxDB, "proxy", reporting)
	}()
	return &CoreProxy{
		globalCtx:  globalCtx,
		conf:       conf,
		rConf:      gConf,
		analyzer:   analyzer,
		apiProxy:   services.NewAPIProxy(conf.GetCoreConf()),
		reqCounter: reqCounter,
		reporting:  reporting,
	}
}
