// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/cncauth"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/backend"
	"apiguard/services/cnc"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type TreqProxy struct {
	*cnc.CoreProxy
	conf               *Conf
	authFallbackCookie *http.Cookie
	reauthSF           singleflight.Group
}

func (tp *TreqProxy) reqUsesMappedSession(req *http.Request) bool {
	if tp.conf.FrontendSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(tp.conf.FrontendSessionCookieName)
	return err == nil
}

func (tp *TreqProxy) AnyPath(ctx *gin.Context) {
	var cached, indirectAPICall bool
	var clientID, humanID common.UserID
	t0 := time.Now().In(tp.GlobalCtx().TimezoneLocation)

	defer func(currUserID, currHumanID *common.UserID, indirect *bool, created time.Time) {
		loggedUserID := currUserID
		if currHumanID.IsValid() && tp.Guard().TestUserIsAnonymous(*currHumanID) {
			loggedUserID = currHumanID
		}
		tp.CountRequest(
			ctx,
			created,
			tp.EnvironConf().ServiceKey,
			*loggedUserID,
		)
		tp.GlobalCtx().BackendLogger.Log(
			ctx.Request,
			tp.EnvironConf().ServiceKey,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			reporting.BackendActionTypeQuery,
		)
	}(&clientID, &humanID, &indirectAPICall, t0)

	if !strings.HasPrefix(ctx.Request.URL.Path, tp.EnvironConf().ServicePath) {
		proxy.WriteError(ctx, fmt.Errorf("invalid path detected"), http.StatusInternalServerError)
		return
	}
	reqProps := tp.Guard().EvaluateRequest(ctx.Request, tp.authFallbackCookie)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user treq/* request")
	clientID = reqProps.ClientID
	if reqProps.ProposedResponse == http.StatusUnauthorized {
		_, err, _ := tp.reauthSF.Do("reauth", func() (any, error) {
			resp := tp.LoginWithToken(tp.conf.CNCAuthToken)
			log.Debug().Msgf("reauthentication result: %s", resp.String())
			if resp.Err() == nil {
				c := resp.Cookie(tp.EnvironConf().CNCAuthCookie)
				cVal := "-"
				if c != nil {
					cVal = c.Value
				}
				log.Debug().
					Str("serviceId", tp.EnvironConf().ServiceKey).
					Str("cookieValue", cVal).
					Msg("performed reauthentication")
				if c != nil {
					tp.authFallbackCookie = c
					return true, nil
				}
				return false, nil
			}
			return false, resp.Err()
		})
		if err != nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf("failed to proxy request: %w", err),
				reqProps.ProposedResponse,
			)
			return
		}
		if tp.authFallbackCookie == nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf(
					"failed to proxy request: cnc auth cookie '%s' not found",
					tp.EnvironConf().CNCAuthCookie,
				),
				reqProps.ProposedResponse,
			)
			return
		}

	} else if reqProps.Error != nil {
		proxy.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %w", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ForbidsAccess() {
		proxy.WriteError(
			ctx,
			errors.New(http.StatusText(reqProps.ProposedResponse)),
			reqProps.ProposedResponse,
		)
		return
	}

	if reqProps.RequiresFallbackCookie {
		log.Debug().
			Str("serviceId", tp.EnvironConf().ServiceKey).
			Str("value", tp.authFallbackCookie.Value).Msg("applying fallback cookie")
		tp.DeleteCookie(ctx.Request, tp.authFallbackCookie.Name)
		ctx.Request.AddCookie(tp.authFallbackCookie)
	}

	passedHeaders := ctx.Request.Header
	if tp.EnvironConf().CNCAuthCookie != tp.conf.FrontendSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.Guard().DetermineTrueUserID(ctx.Request)
		clientID = humanID
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
	}
	passedHeaders[backend.HeaderAPIUserID] = []string{clientID.String()}
	guard.RestrictResponseTime(
		ctx.Writer,
		ctx.Request,
		tp.EnvironConf().ReadTimeoutSecs,
		tp.Guard(),
		common.ClientID{
			IP: ctx.RemoteIP(),
			ID: clientID,
		},
	)

	// first, remap cookie names
	if tp.reqUsesMappedSession(ctx.Request) {
		err := backend.MapFrontendCookieToBackend(
			ctx.Request,
			tp.conf.FrontendSessionCookieName,
			tp.EnvironConf().CNCAuthCookie,
		)
		if err != nil {
			http.Error(
				ctx.Writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}
	// then update auth cookie by x-api-key (if applicable)
	xApiKey := ctx.Request.Header.Get(backend.HeaderAPIKey)
	if xApiKey != "" {
		cookie, err := ctx.Request.Cookie(tp.EnvironConf().CNCAuthCookie)
		if err == nil {
			cookie.Value = xApiKey
		}
	}

	rt0 := time.Now().In(tp.GlobalCtx().TimezoneLocation)
	serviceResp := tp.makeRequest(ctx.Request)
	cached = serviceResp.IsCached()
	tp.WriteReport(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.GetStatusCode(),
		Service:  tp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
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
	defer serviceResp.CloseBodyReader()
	respBody, err := io.ReadAll(serviceResp.GetBodyReader())
	if err != nil {
		log.Error().Err(err).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy request: %s", err),
			http.StatusInternalServerError,
		)
		return
	}
	ctx.Writer.Write(respBody)
}

func (tp *TreqProxy) makeRequest(req *http.Request) proxy.BackendResponse {
	cacheApplCookies := []string{
		tp.conf.FrontendSessionCookieName,
		tp.EnvironConf().CNCAuthCookie,
	}
	resp, err := tp.GlobalCtx().Cache.Get(req, proxy.CachingWithCookies(cacheApplCookies))
	if err == proxy.ErrCacheMiss {
		path := req.URL.Path[len(tp.EnvironConf().ServicePath):]
		resp = tp.ProxyRequest(
			path,
			req.URL.Query(),
			req.Method,
			req.Header,
			req.Body,
		)
		err := tp.GlobalCtx().Cache.Set(req, resp, proxy.CachingWithCookies(cacheApplCookies))
		if err != nil {
			return &proxy.ProxiedResponse{Err: err}
		}
		return resp

	} else if err != nil {
		return &proxy.ProxiedResponse{Err: err}
	}
	return resp
}

func NewTreqProxy(
	globalCtx *globctx.Context,
	conf *Conf,
	gConf *cnc.EnvironConf,
	guard *cncauth.Guard,
	reqCounter chan<- guard.RequestInfo,
) (*TreqProxy, error) {
	proxy, err := cnc.NewCoreProxy(globalCtx, &conf.ProxyConf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create KonText proxy: %w", err)
	}
	return &TreqProxy{
		CoreProxy: proxy,
		conf:      conf,
	}, nil
}
