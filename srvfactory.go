// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/cncauth"
	"apiguard/guard/dflt"
	"apiguard/guard/tlmtr"
	"apiguard/guard/token"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/services/backend/assc"
	"apiguard/services/backend/cja"
	"apiguard/services/backend/gunstick"
	"apiguard/services/backend/hex"
	"apiguard/services/backend/kla"
	"apiguard/services/backend/kontext"
	"apiguard/services/backend/kwords"
	"apiguard/services/backend/lguide"
	"apiguard/services/backend/mquery"
	"apiguard/services/backend/neomat"
	"apiguard/services/backend/psjc"
	"apiguard/services/backend/ssjc"
	"apiguard/services/backend/treq"
	"apiguard/services/cnc"
	"apiguard/services/defaults"
	"apiguard/session"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func InitServices(
	ctx *globctx.Context,
	apiRoutes *gin.RouterGroup,
	globalConf *config.Configuration,
	alarm *monitoring.AlarmTicker,
) error {

	servicesDefaults := make(map[string]defaults.DefaultsProvider)
	sessActions := defaults.NewActions(servicesDefaults)

	// session tools

	apiRoutes.GET("/defaults/:serviceID/:key", sessActions.Get)

	apiRoutes.POST("/defaults/:serviceID/:key", sessActions.Set)

	for sid, servConf := range globalConf.Services {

		switch servConf.Type {

		// "Jazyková příručka ÚJČ"
		case "languageGuide":
			var typedConf lguide.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
			}
			if err := typedConf.Validate("languageGuide"); err != nil {
				return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
			}
			langGuideActions := lguide.NewLanguageGuideActions(
				ctx,
				&typedConf,
				&globalConf.Botwatch,
				globalConf.Telemetry,
				globalConf.ServerReadTimeoutSecs,
				guard,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/language-guide", sid),
				langGuideActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for LanguageGuide enabled")

		// "Akademický slovník současné češtiny"
		case "assc":
			var typedConf assc.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
			}
			if err := typedConf.Validate("assc"); err != nil {
				return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
			}
			asscActions := assc.NewASSCActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/assc", sid),
				asscActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for ASSC enabled")

		// "Slovník spisovného jazyka českého"
		case "ssjc":
			var typedConf ssjc.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
			}
			if err := typedConf.Validate("ssjc"); err != nil {
				return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
			}
			ssjcActions := ssjc.NewSSJCActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/ssjc", sid),
				ssjcActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for SSJC enabled")

		// "Příruční slovník jazyka českého"
		case "psjc":
			var typedConf psjc.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
			}
			if err := typedConf.Validate("psjc"); err != nil {
				return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
			}
			psjcActions := psjc.NewPSJCActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/psjc", sid),
				psjcActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for PSJC enabled")

		// "Kartotéka lexikálního archivu"
		case "kla":
			var typedConf kla.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
			}
			if err := typedConf.Validate("kla"); err != nil {
				return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
			}
			klaActions := kla.NewKLAActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/kla", sid),
				klaActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for KLA enabled")

		// "Neomat"
		case "neomat":
			var typedConf neomat.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
			}
			if err := typedConf.Validate("neomat"); err != nil {
				return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
			}
			neomatActions := neomat.NewNeomatActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/neomat", sid),
				neomatActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for Neomat enabled")

		// "Český jazykový atlas"
		case "cja":
			var typedConf cja.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
			}
			if err := typedConf.Validate("cja"); err != nil {
				return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
			}
			guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
			}
			cjaActions := cja.NewCJAActions(
				ctx,
				&typedConf,
				guard,
				globalConf.ServerReadTimeoutSecs,
			)
			apiRoutes.GET(
				fmt.Sprintf("/service/%d/cja", sid),
				cjaActions.Query,
			)
			log.Info().Int("sid", sid).Msg("Proxy for CJA enabled")

		// KonText (API) proxy
		case "kontext":
			var typedConf kontext.Conf
			if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
				return fmt.Errorf("failed to initialize service %d (kontext): %w", sid, err)
			}
			if err := typedConf.Validate(fmt.Sprintf("%d/kontext", sid)); err != nil {
				return fmt.Errorf("failed to initialize service %d (kontext): %w", sid, err)
			}
			kontextGuard := cncauth.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.ExternalSessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)

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
				&typedConf.ProxyConf,
				&cnc.EnvironConf{
					CNCAuthCookie:     globalConf.CNCAuth.SessionCookieName,
					AuthTokenEntry:    authTokenEntry,
					ServicePath:       fmt.Sprintf("/service/%d/kontext", sid),
					ServiceName:       "kontext",
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
				},
				kontextGuard,
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

					} else {
						kontextActions.AnyPath(ctx)
					}
				},
			)
			servicesDefaults[fmt.Sprintf("%d/kontext", sid)] = kontextActions
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
					ServiceName:       "mquery",
					CNCPortalLoginURL: cncPortalLoginURL,
					ReadTimeoutSecs:   globalConf.ServerReadTimeoutSecs,
				},
				grd,
				mqueryReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (mquery): %w", sid, err)
			}

			apiRoutes.Any(
				fmt.Sprintf("/service/%d/mquery/*path", sid),
				func(ctx *gin.Context) {
					if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
						mqueryActions.Login(ctx)

					} else if ctx.Param("path") == "/preflight" {
						mqueryActions.Preflight(ctx)

					} else {
						mqueryActions.AnyPath(ctx)
					}
				},
			)
			log.Info().Int("sid", sid).Msg("Proxy for MQuery enabled")

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
				typedConf.ExternalSessionCookieName,
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
				sid,
				globalConf.CNCAuth.SessionCookieName,
				cnca,
				globalConf.ServerReadTimeoutSecs,
				treqReqCounter,
			)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (treq): %w", sid, err)
			}
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/treq/*path", sid),
				treqActions.AnyPath,
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
			analyzer := dflt.New(
				ctx,
				globalConf.CNCAuth.SessionCookieName,
				typedConf.SessionValType,
				typedConf.Limits,
			)
			go analyzer.Run()
			internalURL, err := url.Parse(typedConf.InternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			externalURL, err := url.Parse(typedConf.ExternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}
			coreProxy, err := proxy.NewAPIProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (kwords): %w", sid, err)
			}

			kwordsActions := proxy.NewPublicAPIProxy(
				coreProxy,
				sid,
				client,
				analyzer.ExposeAsCounter(),
				analyzer,
				ctx.CNCDB,
				proxy.PublicAPIProxyOpts{
					ServiceName:      "kwords",
					InternalURL:      internalURL,
					ExternalURL:      externalURL,
					AuthCookieName:   globalConf.CNCAuth.SessionCookieName,
					UserIDHeaderName: typedConf.TrueUserIDHeader,
					ReadTimeoutSecs:  globalConf.ServerReadTimeoutSecs,
				},
			)
			apiRoutes.Any(
				"/service/kwords/*path",
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
			internalURL, err := url.Parse(typedConf.InternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			externalURL, err := url.Parse(typedConf.ExternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			coreProxy, err := proxy.NewAPIProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (gunstick): %w", sid, err)
			}
			gunstickActions := proxy.NewPublicAPIProxy(
				coreProxy,
				sid,
				client,
				grd.ExposeAsCounter(),
				grd,
				ctx.CNCDB,
				proxy.PublicAPIProxyOpts{
					ServiceName:     "gunstick",
					InternalURL:     internalURL,
					ExternalURL:     externalURL,
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
			internalURL, err := url.Parse(typedConf.InternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			externalURL, err := url.Parse(typedConf.ExternalURL)
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			coreProxy, err := proxy.NewAPIProxy(typedConf.GetCoreConf())
			if err != nil {
				return fmt.Errorf("failed to initialize service %d (hex): %w", sid, err)
			}
			hexActions := proxy.NewPublicAPIProxy(
				coreProxy,
				sid,
				client,
				grd.ExposeAsCounter(),
				grd,
				ctx.CNCDB,
				proxy.PublicAPIProxyOpts{
					ServiceName:     "hex",
					InternalURL:     internalURL,
					ExternalURL:     externalURL,
					ReadTimeoutSecs: globalConf.ServerReadTimeoutSecs,
				},
			)
			apiRoutes.Any(
				fmt.Sprintf("/service/%d/hex/*path", sid),
				hexActions.AnyPath,
			)
			log.Info().Int("sid", sid).Msg("Proxy for Hex enabled")

		default:
			log.Warn().Msgf("Ignoring unknown service %d: %s", sid, servConf.Type)
		}
	}
	return nil
}
