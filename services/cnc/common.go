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

func (kp *Proxy) LogRequest(ctx *gin.Context, currHumanID *common.UserID, indirect *bool, cached *bool, created time.Time) {
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

func (kp *Proxy) MonitoringWrite(item reporting.Timescalable) {
	kp.tDBWriter.Write(item)
}

func (kp *Proxy) reqUsesMappedSession(req *http.Request) bool {
	if kp.conf.FrontendSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(kp.conf.FrontendSessionCookieName)
	return err == nil
}

func (kp *Proxy) ProcessReqHeaders(
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

func (kp *Proxy) AuthorizeRequestOrRespondErr(ctx *gin.Context) (guard.ReqEvaluation, bool) {
	reqProps := kp.guard.EvaluateRequest(ctx.Request, nil)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user * request")
	if reqProps.Error != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request")
		proxy.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return guard.ReqEvaluation{}, false

	} else if reqProps.ForbidsAccess() {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedResponse), reqProps.ProposedResponse)
		return guard.ReqEvaluation{}, false
	}
	return reqProps, true
}

func (kp *Proxy) HandleRequest(
	req *http.Request,
	reqProps guard.ReqEvaluation,
	useCache bool,
) proxy.ResponseProcessor {
	kp.debugLogRequest(req)
	cacheApplCookies := make([]string, 0, 2)
	if kp.conf.CachingPerSession {
		cacheApplCookies = append(cacheApplCookies, kp.rConf.CNCAuthCookie, kp.conf.FrontendSessionCookieName)
	}
	var respHandler proxy.ResponseProcessor
	if useCache {
		respHandler = kp.FromCache(
			req,
			proxy.CachingWithCookies(cacheApplCookies),
		)

	} else {
		respHandler = proxy.NewDirectResponse(nil)
	}
	if respHandler.Error() != nil {
		return respHandler
	}
	respHandler.HandleCacheMiss(func() proxy.BackendResponse {
		resp := kp.apiProxy.Request(
			// TODO use some path builder here
			path.Join("/", req.URL.Path[len(kp.rConf.ServicePath):]),
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		kp.debugLogResponse(req, resp)
		return resp
	})
	return respHandler
}

func (kp *Proxy) MakeStreamRequest(
	req *http.Request,
	reqProps guard.ReqEvaluation,
) proxy.ResponseProcessor {
	kp.debugLogRequest(req)
	cacheApplCookies := make([]string, 0, 2)
	if kp.conf.CachingPerSession {
		cacheApplCookies = append(cacheApplCookies, kp.rConf.CNCAuthCookie, kp.conf.FrontendSessionCookieName)
	}
	resp := kp.FromCache(
		req,
		proxy.CachingWithCookies(cacheApplCookies),
	)
	resp.HandleCacheMiss(func() proxy.BackendResponse {
		return kp.apiProxy.Request(
			// TODO use some path builder here
			path.Join("/", req.URL.Path[len(kp.rConf.ServicePath):]),
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
	})
	return resp
}

// MakeCacheablePOSTRequest performs a HTTP request which is expected
// to be POST but the method will cache the result. It requires explicit
// passing of request body data leaving their extraction up to the consumer.
// This should prevent unwanted reading of a body from `req` which would
// lead to an empty body during backend requesting (typically, once inner
// io.Reader is read, there is no way one can read the body again so we must
// preserve it in requests in the original state and make copies of it only
// in special occasions as it affects performance.
//
// In case the request is not POST, the method panics.
func (kp *Proxy) MakeCacheablePOSTRequest(
	req *http.Request,
	reqProps guard.ReqEvaluation,
	reqBody []byte,
) proxy.ResponseProcessor {
	if req.Method != http.MethodPost {
		panic("assertion in MakeCacheablePOSTRequest error: req is not POST")
	}
	kp.debugLogRequest(req)
	cacheApplCookies := make([]string, 0, 2)
	if kp.conf.CachingPerSession {
		cacheApplCookies = append(cacheApplCookies, kp.rConf.CNCAuthCookie, kp.conf.FrontendSessionCookieName)
	}
	resp := kp.FromCache(
		req,
		proxy.CachingWithCookies(cacheApplCookies),
		proxy.CachingWithReqBody(reqBody),
		proxy.CachingWithCacheablePOST(),
	)
	resp.HandleCacheMiss(func() proxy.BackendResponse {
		backendResp := kp.apiProxy.Request(
			// TODO use some path builder here
			path.Join("/", req.URL.Path[len(kp.rConf.ServicePath):]),
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		kp.debugLogResponse(req, backendResp)
		return backendResp
	})
	return resp
}
