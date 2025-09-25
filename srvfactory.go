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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/cncauth"
	"github.com/czcorpus/apiguard/guard/dflt"
	"github.com/czcorpus/apiguard/guard/null"
	"github.com/czcorpus/apiguard/guard/token"
	"github.com/czcorpus/apiguard/monitoring"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/proxy/public"
	"github.com/czcorpus/apiguard/services/backend/frodo"
	"github.com/czcorpus/apiguard/services/backend/gunstick"
	"github.com/czcorpus/apiguard/services/backend/hex"
	"github.com/czcorpus/apiguard/services/backend/kontext"
	"github.com/czcorpus/apiguard/services/backend/kwords"
	"github.com/czcorpus/apiguard/services/backend/mquery"
	"github.com/czcorpus/apiguard/services/backend/scollex"
	"github.com/czcorpus/apiguard/services/backend/treq"
	"github.com/czcorpus/apiguard/services/backend/wss"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/apiguard/services/ujc"
	"github.com/czcorpus/apiguard/session"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func AfterHandlerCallback(callback func(c *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Code executed before the handler (if needed)
		c.Next() // Process the request by calling subsequent handlers
		// Code executed after the handler finishes
		callback(c)
	}
}

func InitServices(
	ctx *globctx.Context,
	engine http.Handler,
	apiRoutes *gin.RouterGroup,
	globalConf *config.Configuration,
	alarm *monitoring.AlarmTicker,
) error {

	for sid, servConf := range globalConf.Services {

		switch servConf.Type {

		// ÚJČ services
		case "languageGuide", "assc", "ssjc", "psjc", "kla", "neomat", "cja":
			if err := ujc.InitUJCService(ctx, sid, servConf, globalConf, apiRoutes); err != nil {
				return err
			}

		// KonText (API) proxy
		case "kontext":
			var typedConf kontext.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (kontext): %w", sid, err)
			}
			if err := typedConf.Validate(fmt.Sprintf("%d/kontext", sid)); err != nil {
				return fmt.Errorf("failed to initialize service %d (kontext): %w", sid, err)
			}
			var cncGuard guard.ServiceGuard
			switch typedConf.SessionValType {
			case session.SessionTypeNone:
				cncGuard = &null.Guard{}
				log.Warn().
					Msgf("service %d/kontext is running in 'null' session mode - this is mostly for testing", sid)
			case session.SessionTypeCNC:
				cncGuard = cncauth.New(
					ctx,
					globalConf.CNCAuth.SessionCookieName,
					typedConf.FrontendSessionCookieName,
					typedConf.SessionValType,
					typedConf.Limits,
				)
			default:
				return fmt.Errorf(
					"failed to initialize service %d/kontext: service does not support session type %s",
					sid, typedConf.SessionValType,
				)
			}

			var kontextReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				kontextReqCounter = alarm.Register(
					fmt.Sprintf("%d/kontext", sid),
					typedConf.Alarm,
					typedConf.Limits,
				)
			}
			kontextActions, err := kontext.NewKontextProxy(
				ctx,
				&typedConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/kontext", sid),
					ServiceKey:        fmt.Sprintf("%d/kontext", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				cncGuard,
				kontextReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kontext): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/kontext/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "/login" && ctx.Request.Method == http.MethodPost {
						kontextActions.Login(ctx)

					} else if ctx.Param("path") == "/preflight" {
						kontextActions.Preflight(ctx)

					} else if ctx.Param("path") == "/query_submit" {
						kontextActions.QuerySubmitAndView(ctx)

					} else {
						kontextActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for Kontext enabled")

		// MQuery proxy
		case "mquery":
			var typedConf mquery.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (mquery): %w", sid, err)
			}
			// we don't want to bother admins with none session type, so we set it here
			typedConf.SessionValType = session.SessionTypeNone
			if err := typedConf.Validate("mquery"); err != nil {
				return fmt.Errorf("failed to initialize service %d (mquery): %w", sid, err)
			}
			var mqueryReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				mqueryReqCounter = alarm.Register(
					fmt.Sprintf("%d/mquery", sid),
					typedConf.Alarm,
					typedConf.Limits,
				)
			}
			var grd guard.ServiceGuard
			switch typedConf.GuardType {
			case guard.GuardTypeToken:
				grd = token.NewGuard(
					ctx,
					fmt.Sprintf("/service/%d/mquery", sid),
					typedConf.TokenHeaderName,
					typedConf.Limits,
					typedConf.Tokens,
					[]string{"/openapi"},
				)
			case guard.GuardTypeDflt:
				grd = dflt.New(
					ctx,
					globalConf.CNCAuth.SessionCookieName,
					typedConf.SessionValType,
					typedConf.Limits,
				)
			default:
				return fmt.Errorf("MQuery proxy does not support guard type `%s`", typedConf.GuardType)
			}

			mqueryActions, err := mquery.NewMQueryProxy(
				ctx,
				&typedConf.ProxyConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/mquery", sid),
					ServiceKey:        fmt.Sprintf("%d/mquery", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				grd,
				mqueryReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (mquery): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/mquery/*path", sid),
				AfterHandlerCallback(func(ctx *gin.Context) {
					ctx.Writer.Flush() // this is important for data streaming mode
					// when we use custom writer and this call closes channel which
					// signals that a sub-stream is finished
				}),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
						mqueryActions.Login(ctx)

					} else if ctx.Param("path") == "/preflight" {
						mqueryActions.Preflight(ctx)

					} else if ctx.Param("path") == "/merge-freqs" {
						mqueryActions.MergeFreqs(ctx)

					} else if ctx.Param("path") == "/speeches" {
						mqueryActions.Speeches(ctx)

					} else if ctx.Param("path") == "/time-dist-word" {
						mqueryActions.TimeDistAltWord(ctx)

					} else {
						mqueryActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for MQuery enabled")

		// Frodo proxy
		case "frodo":
			var typedConf frodo.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (frodo): %w", sid, err)
			}
			// we don't want to bother admins with none session type, so we set it here
			typedConf.SessionValType = session.SessionTypeNone
			if err := typedConf.Validate("frodo"); err != nil {
				return fmt.Errorf("failed to initialize service %d (frodo): %w", sid, err)
			}
			var frodoReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				frodoReqCounter = alarm.Register(
					fmt.Sprintf("%d/frodo", sid),
					typedConf.Alarm,
					typedConf.Limits,
				)
			}
			var grd guard.ServiceGuard
			switch typedConf.GuardType {
			case guard.GuardTypeToken:
				grd = token.NewGuard(
					ctx,
					fmt.Sprintf("/service/%d/frodo", sid),
					typedConf.TokenHeaderName,
					typedConf.Limits,
					typedConf.Tokens,
					[]string{"/openapi"},
				)
			case guard.GuardTypeDflt:
				grd = dflt.New(
					ctx,
					globalConf.CNCAuth.SessionCookieName,
					typedConf.SessionValType,
					typedConf.Limits,
				)
			default:
				return fmt.Errorf("frodo proxy does not support guard type `%s`", typedConf.GuardType)
			}

			frodoActions, err := frodo.NewFrodoProxy(
				ctx,
				&typedConf.ProxyConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/frodo", sid),
					ServiceKey:        fmt.Sprintf("%d/frodo", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				grd,
				frodoReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (frodo): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/frodo/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
						frodoActions.Login(ctx)

					} else if ctx.Param("path") == "/preflight" {
						frodoActions.Preflight(ctx)

					} else {
						frodoActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for Frodo enabled")

		// Treq (API) proxy
		case "treq":
			var typedConf treq.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (treq): %w", sid, err)
			}
			if err := typedConf.Validate("treq"); err != nil {
				return fmt.Errorf("failed to initialize service %d (treq): %w", sid, err)
			}
			cnca := cncauth.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.FrontendSessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			var treqReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				treqReqCounter = alarm.Register(
					fmt.Sprintf("%d/treq", sid), typedConf.Alarm, typedConf.Limits)
			}
			treqActions, err := treq.NewTreqProxy(
				ctx,
				&typedConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/treq", sid),
					ServiceKey:        fmt.Sprintf("%d/treq", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				cnca,
				engine,
				treqReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (treq): %w", sid, err)
			}
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/treq/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "/api/v1/subsets" {
						treqActions.Subsets(ctx)

					} else if ctx.Param("path") == "/api/v1/with-examples" {
						treqActions.WithExamples(ctx)

					} else {
						treqActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for Treq enabled")

		// KWords (API) proxy
		case "kwords":
			var typedConf kwords.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			if err := typedConf.Validate("kwords"); err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			client := httpclient.New(
				httpclient.WithFollowRedirects(),
				httpclient.WithInsecureSkipVerify(),
				httpclient.WithIdleConnTimeout(time.Duration(60)*time.Second),
			)

			if typedConf.GuardType != guard.GuardTypeDflt {
				return fmt.Errorf("failed to initialize service %d (kwords): unsupported guard type %s (supported: dflt)", sid, typedConf.GuardType)
			}

			analyzer := dflt.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			go analyzer.Run()
			backendURL, err := url.Parse(typedConf.BackendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			frontendUrl, err := url.Parse(typedConf.FrontendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			coreProxy, err := proxy.NewCoreProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}

			kwordsActions := public.NewProxy(
				ctx,
				coreProxy,
				sid,
				client,
				analyzer.ExposeAsCounter(),
				analyzer,
				public.PublicAPIProxyOpts{
					ServiceKey:       fmt.Sprintf("%d/kwords", sid),
					ServicePath:      fmt.Sprintf("/service/%d/kwords", sid),
					BackendURL:       backendURL,
					FrontendURL:      frontendUrl,
					AuthCookieName:   globalConf.CNCAuth.SessionCookieName,
					UserIDHeaderName: typedConf.TrueUserIDHeader,
					ReadTimeoutSecs:  globalConf.ServerReadTimeoutSecs,
				},
			)
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/kwords/*path", sid),
				kwordsActions.AnyPath)
			log.Info().Int("sid", sid).Msg("Proxy for KWords enabled")

		// Gunstick proxy
		case "gunstick":
			var typedConf gunstick.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			if err := typedConf.Validate("gunstick"); err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			client := httpclient.New(
				httpclient.WithFollowRedirects(),
				httpclient.WithInsecureSkipVerify(),
				httpclient.WithIdleConnTimeout(time.Duration(60)*time.Second),
			)
			grd := dflt.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			go grd.Run()
			backendURL, err := url.Parse(typedConf.BackendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			frontendURL, err := url.Parse(typedConf.FrontendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			coreProxy, err := proxy.NewCoreProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			gunstickActions := public.NewProxy(
				ctx,
				coreProxy,
				sid,
				client,
				grd.ExposeAsCounter(),
				grd,
				public.PublicAPIProxyOpts{
					ServicePath:     fmt.Sprintf("/service/%d/gunstick", sid),
					ServiceKey:      fmt.Sprintf("%d/gunstick", sid),
					BackendURL:      backendURL,
					FrontendURL:     frontendURL,
					ReadTimeoutSecs: globalConf.ServerReadTimeoutSecs,
				},
			)
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/gunstick/*path", sid),
				gunstickActions.AnyPath,
			)
			log.Info().Int("sid", sid).Msg("Proxy for Gunstick enabled")

		// Hex proxy
		case "hex":
			var typedConf hex.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			if err := typedConf.Validate("hex"); err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			client := httpclient.New(
				httpclient.WithFollowRedirects(),
				httpclient.WithInsecureSkipVerify(),
				httpclient.WithIdleConnTimeout(time.Duration(60)*time.Second),
			)
			grd := dflt.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			go grd.Run()
			backendURL, err := url.Parse(typedConf.BackendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			frontendURL, err := url.Parse(typedConf.FrontendURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			coreProxy, err := proxy.NewCoreProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			hexActions := public.NewProxy(
				ctx,
				coreProxy,
				sid,
				client,
				grd.ExposeAsCounter(),
				grd,
				public.PublicAPIProxyOpts{
					ServicePath:         fmt.Sprintf("/service/%d/hex", sid),
					ServiceKey:          fmt.Sprintf("%d/hex", sid),
					BackendURL:          backendURL,
					FrontendURL:         frontendURL,
					ReadTimeoutSecs:     globalConf.ServerReadTimeoutSecs,
					ResponseInterceptor: hex.Interceptor,
				},
			)
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/hex/*path", sid),
				hexActions.AnyPath,
			)
			log.Info().Int("sid", sid).Msg("Proxy for Hex enabled")

		// WSS proxy
		case "wss":
			var typedConf wss.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (wss): %w", sid, err)
			}
			if err := typedConf.Validate("wss"); err != nil {
				return fmt.Errorf("failed to initialize service %d (wss): %w", sid, err)
			}
			analyzer := dflt.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			go analyzer.Run()

			var wssReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				wssReqCounter = alarm.Register(
					fmt.Sprintf("%d/mquery", sid),
					typedConf.Alarm,
					typedConf.Limits,
				)
			}

			var grd guard.ServiceGuard
			switch typedConf.GuardType {
			case guard.GuardTypeToken:
				grd = token.NewGuard(
					ctx,
					fmt.Sprintf("/service/%d/wss", sid),
					typedConf.TokenHeaderName,
					typedConf.Limits,
					typedConf.Tokens,
					[]string{"/openapi"},
				)
			case guard.GuardTypeDflt:
				grd = dflt.New(
					ctx,
					globalConf.CNCAuth.SessionCookieName,
					typedConf.SessionValType,
					typedConf.Limits,
				)
			default:
				return fmt.Errorf("MQuery proxy does not support guard type `%s`", typedConf.GuardType)
			}

			wssActions, err := wss.NewWSServerProxy(
				ctx,
				&typedConf.ProxyConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/wss", sid),
					ServiceKey:        fmt.Sprintf("%d/wss", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				grd,
				engine,
				wssReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (wss): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/wss/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "/collocations-tt" && ctx.Request.Method == http.MethodPost {
						wssActions.CollocationsTT(ctx)

					} else {
						wssActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for wss enabled")

		// Scollex proxy
		case "scollex":
			var typedConf scollex.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (scollex): %w", sid, err)
			}
			// we don't want to bother admins with none session type, so we set it here
			typedConf.SessionValType = session.SessionTypeNone
			if err := typedConf.Validate("scollex"); err != nil {
				return fmt.Errorf("failed to initialize service %d (scollex): %w", sid, err)
			}
			var scollexReqCounter chan<- guard.RequestInfo
			if len(typedConf.Limits) > 0 {
				scollexReqCounter = alarm.Register(
					fmt.Sprintf("%d/scollex", sid),
					typedConf.Alarm,
					typedConf.Limits,
				)
			}
			var grd guard.ServiceGuard
			switch typedConf.GuardType {
			case guard.GuardTypeToken:
				grd = token.NewGuard(
					ctx,
					fmt.Sprintf("/service/%d/scollex", sid),
					typedConf.TokenHeaderName,
					typedConf.Limits,
					typedConf.Tokens,
					[]string{"/openapi"},
				)
			case guard.GuardTypeDflt:
				grd = dflt.New(
					ctx,
					globalConf.CNCAuth.SessionCookieName,
					typedConf.SessionValType,
					typedConf.Limits,
				)
			default:
				return fmt.Errorf("scollex proxy does not support guard type `%s`", typedConf.GuardType)
			}

			scollexActions, err := scollex.NewScollexProxy(
				ctx,
				&typedConf.ProxyConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/scollex", sid),
					ServiceKey:        fmt.Sprintf("%d/scollex", sid),
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
					IsStreamingMode:   globalConf.OperationMode == config.OperationModeStreaming,
				},
				grd,
				scollexReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (scollex): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/scollex/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
						scollexActions.Login(ctx)

					} else {
						scollexActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for Scollex enabled")

		default:
			log.Warn().Msgf("Ignoring unknown service %d: %s", sid, servConf.Type)
		}
	}
	return nil
}
