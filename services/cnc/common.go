// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cnc

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/backend"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (kp *CoreProxy) LogRequest(ctx *gin.Context, currHumanID *common.UserID, indirect *bool, cached *bool, created time.Time) {
	if kp.reqCounter != nil {
		kp.reqCounter <- guard.RequestInfo{
			Created:     created,
			Service:     kp.rConf.ServiceKey,
			NumRequests: 1,
			UserID:      *currHumanID,
			IP:          ctx.ClientIP(),
		}
	}
	kp.globalCtx.BackendLogger.Log(
		ctx.Request,
		kp.rConf.ServiceKey,
		time.Since(created),
		*cached,
		*currHumanID,
		*indirect,
		reporting.BackendActionTypeQuery,
	)
}

func (kp *CoreProxy) MonitoringWrite(item reporting.Timescalable) {
	kp.tDBWriter.Write(item)
}

func (kp *CoreProxy) reqUsesMappedSession(req *http.Request) bool {
	if kp.conf.FrontendSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(kp.conf.FrontendSessionCookieName)
	return err == nil
}

func (kp *CoreProxy) ProcessReqHeaders(
	ctx *gin.Context,
	humanID, userID common.UserID,
	indirectAPICall *bool,
) error {
	passedHeaders := ctx.Request.Header

	if ctx.Request.Header.Get("host") == "" {
		ctx.Request.Header.Set("host", kp.frontendHost)
	}

	if kp.rConf.CNCAuthCookie != kp.conf.FrontendSessionCookieName {
		passedHeaders[backend.HeaderAPIUserID] = []string{humanID.String()}

	} else {
		passedHeaders[backend.HeaderAPIUserID] = []string{userID.String()}
	}

	if passedHeaders.Get(backend.HeaderIndirectCall) != "" {
		*indirectAPICall = true
	}

	if kp.conf.TrueUserIDHeader != "" {
		passedHeaders[kp.conf.TrueUserIDHeader] = []string{userID.String()}
	}

	if kp.conf.UseHeaderXApiKey {
		if kp.reqUsesMappedSession(ctx.Request) {
			passedHeaders[backend.HeaderAPIKey] = []string{
				proxy.GetCookieValue(ctx.Request, kp.conf.FrontendSessionCookieName),
			}

		} else {
			passedHeaders[backend.HeaderAPIKey] = []string{
				proxy.GetCookieValue(ctx.Request, kp.rConf.CNCAuthCookie),
			}
		}

	} else if kp.reqUsesMappedSession(ctx.Request) {

		err := backend.MapFrontendCookieToBackend(
			ctx.Request,
			kp.conf.FrontendSessionCookieName,
			kp.rConf.CNCAuthCookie,
		)
		if err != nil {
			return fmt.Errorf("CoreProxy failed to process headers: %w", err)
		}
	}
	return nil
}

func (kp *CoreProxy) AuthorizeRequestOrRespondErr(ctx *gin.Context) (guard.ReqEvaluation, bool) {
	reqProps := kp.guard.EvaluateRequest(ctx.Request)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user * request")
	if reqProps.Error != nil {
		// TODO
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request")
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return guard.ReqEvaluation{}, false

	} else if reqProps.ForbidsAccess() {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedResponse), reqProps.ProposedResponse)
		return guard.ReqEvaluation{}, false
	}
	return reqProps, true
}

func (kp *CoreProxy) MakeRequest(
	req *http.Request,
	reqProps guard.ReqEvaluation,
) proxy.BackendResponse {
	kp.debugLogRequest(req)
	cacheApplCookies := []string{kp.rConf.CNCAuthCookie, kp.conf.FrontendSessionCookieName}
	resp, err := kp.globalCtx.Cache.Get(req, cacheApplCookies)
	if err == proxy.ErrCacheMiss {
		resp = kp.apiProxy.Request(
			// TODO use some path builder here
			path.Join("/", req.URL.Path[len(kp.rConf.ServicePath):]),
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		kp.debugLogResponse(req, resp, err)
		err = kp.globalCtx.Cache.Set(req, resp, cacheApplCookies)
		if err != nil {
			resp = &proxy.ProxiedResponse{Err: err}
		}
		return resp
	}
	if err != nil {
		return &proxy.ProxiedResponse{Err: err}
	}
	return resp
}
