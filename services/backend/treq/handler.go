// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package treq

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/cncauth"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/reporting"
	"github.com/czcorpus/apiguard/services/backend"
	"github.com/czcorpus/apiguard/services/cnc"

	"github.com/czcorpus/cnc-gokit/util"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type subsetArgs struct {
	From            string   `json:"from"`
	To              string   `json:"to"`
	MultiWord       bool     `json:"multiword"`
	Regex           bool     `json:"regex"`
	Lemma           bool     `json:"lemma"`
	CaseInsensitive bool     `json:"ci"`
	Pkgs            []string `json:"pkgs[i]"`
	Query           string   `json:"query"`
	Asc             bool     `json:"asc"`
	Order           string   `json:"order"`
}

func (args subsetArgs) ToQuery() url.Values {
	urlArgs := make(url.Values)
	urlArgs.Add("from", args.From)
	urlArgs.Add("to", args.To)
	urlArgs.Add("multiword", util.Ternary(args.MultiWord, "true", "false"))
	urlArgs.Add("lemma", util.Ternary(args.Lemma, "true", "false"))
	urlArgs.Add("ci", util.Ternary(args.CaseInsensitive, "true", "false"))
	for _, pk := range args.Pkgs {
		urlArgs.Add("pkgs[i]", pk)
	}
	urlArgs.Add("query", args.Query)
	urlArgs.Add("asc", util.Ternary(args.Asc, "true", "false"))
	urlArgs.Add("order", args.Order)
	return urlArgs
}

type subsetsReq map[string]subsetArgs

type TreqProxy struct {
	*cnc.Proxy
	conf               *Conf
	authFallbackCookie *http.Cookie
	reauthSF           singleflight.Group
	httpEngine         http.Handler
}

func (tp *TreqProxy) reqUsesMappedSession(req *http.Request) bool {
	if tp.conf.FrontendSessionCookieName == "" {
		return false
	}
	_, err := req.Cookie(tp.conf.FrontendSessionCookieName)
	return err == nil
}

func (tp *TreqProxy) AnyPath(ctx *gin.Context) {
	fmt.Println("ANYPATH TREQ, req headers: ", ctx.Request.Header)
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
		tp.GlobalCtx().BackendLoggers[tp.EnvironConf().ServiceKey].Log(
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
	serviceResp := tp.HandleRequest(ctx.Request, reqProps, true)
	cached = serviceResp.IsCacheHit()
	tp.WriteReport(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.Response().GetStatusCode(),
		Service:  tp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
	if serviceResp.Error() != nil {
		log.Error().Err(serviceResp.Error()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.Error()),
			http.StatusInternalServerError,
		)
		return
	}

	for k, v := range serviceResp.Response().GetHeaders() {
		ctx.Writer.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	serviceResp.WriteResponse(ctx.Writer)
}

func NewTreqProxy(
	globalCtx *globctx.Context,
	conf *Conf,
	gConf *cnc.EnvironConf,
	guard *cncauth.Guard,
	httpEngine http.Handler,
	reqCounter chan<- guard.RequestInfo,
) (*TreqProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, &conf.ProxyConf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create KonText proxy: %w", err)
	}
	return &TreqProxy{
		Proxy:      proxy,
		conf:       conf,
		httpEngine: httpEngine,
	}, nil
}
