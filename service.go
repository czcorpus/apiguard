// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/common"
	"apiguard/config"
	"apiguard/guard"
	"apiguard/guard/dflt"
	"apiguard/guard/sessionmap"
	"apiguard/guard/tlmtr"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/backend/assc"
	"apiguard/services/backend/cja"
	"apiguard/services/backend/kla"
	"apiguard/services/backend/kontext"
	"apiguard/services/backend/lguide"
	"apiguard/services/backend/mquery"
	"apiguard/services/backend/neomat"
	"apiguard/services/backend/psjc"
	"apiguard/services/backend/ssjc"
	"apiguard/services/backend/treq"
	"apiguard/services/cnc"
	"apiguard/services/defaults"
	"apiguard/services/tstorage"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	cncPortalLoginURL = "https://www.korpus.cz/login"
	authTokenEntry    = "personal_access_token"
)

func runService(conf *config.Configuration) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, syscall.SIGHUP)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	tDBWriter := createTDBWriter(ctx, conf.Reporting, conf.TimezoneLocation())
	globalCtx := createGlobalCtx(ctx, conf, tDBWriter)

	reloadChan := make(chan bool)
	go func() {
		for evt := range syscallChan {
			log.Warn().Str("signalName", evt.String()).Msg("received OS signal")
			if evt == syscall.SIGHUP {
				reloadChan <- true
			}
		}
		close(reloadChan)
	}()

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	apiRoutes := engine.Group("/")
	apiRoutes.Use(uniresp.AlwaysJSONContentType())
	apiRoutes.Use(func(ctx *gin.Context) {
		if !conf.IPAllowedForAPI(net.ParseIP(ctx.ClientIP())) {
			ctx.AbortWithStatusJSON(
				http.StatusUnauthorized,
				map[string]string{"status": http.StatusText(http.StatusUnauthorized)},
			)
			return
		}
		ctx.Next()
	})

	publicRoutes := engine.Group("/")
	publicRoutes.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/html")
		c.Next()
	})

	// alarm
	alarm := monitoring.NewAlarmTicker(
		globalCtx,
		conf.TimezoneLocation(),
		conf.Mail,
		conf.PublicRoutesURL,
		conf.Monitoring,
	)
	go alarm.Run(reloadChan)

	if !conf.IgnoreStoredState {
		err := monitoring.LoadState(alarm)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("Failed to load alarm status from disk. Please use -ignore-stored-state to " +
					"skip the action or remove the problematic file.")
		}
	}

	apiRoutes.GET("/alarm", alarm.HandleReportListAction)
	apiRoutes.GET("/alarms/list", alarm.HandleListAction)
	apiRoutes.POST("/alarms/clean", alarm.HandleCleanAction)

	// ----------------------

	var pingReqCounter chan<- guard.RequestInfo

	if len(conf.Services.Kontext.Limits) > 0 {
		pingReqCounter = alarm.Register(
			"ping",
			monitoring.AlarmConf{
				Recipients:                   []string{},
				RecCounterCleanupProbability: 0.5,
			},
			[]proxy.Limit{
				{
					ReqPerTimeThreshold:     10,
					ReqCheckingIntervalSecs: 10,
				},
				{
					ReqPerTimeThreshold:     20,
					ReqCheckingIntervalSecs: 60,
				},
			},
		)
	}

	apiRoutes.GET("/service/ping", func(ctx *gin.Context) {
		t0 := time.Now()
		defer func() {
			globalCtx.BackendLogger.Log(
				ctx.Request,
				"ping",
				time.Since(t0),
				false,
				common.InvalidUserID,
				false,
				reporting.BackendActionTypeQuery,
			)
		}()
		globalCtx.ReportingWriter.Write(&PingReport{
			DateTime: time.Now(),
			Status:   200,
		})
		pingReqCounter <- guard.RequestInfo{
			Created:     time.Now().In(conf.TimezoneLocation()),
			Service:     "ping",
			NumRequests: 1,
			UserID:      1,
			IP:          ctx.ClientIP(),
		}
		uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"ok": true})
	})

	// --------------------

	// "Jazyková příručka ÚJČ"

	if conf.Services.LanguageGuide.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for LanguageGuide")
			return
		}
		langGuideActions := lguide.NewLanguageGuideActions(
			globalCtx,
			&conf.Services.LanguageGuide,
			&conf.Botwatch,
			conf.Telemetry,
			conf.ServerReadTimeoutSecs,
			guard,
		)
		apiRoutes.GET("/service/language-guide", langGuideActions.Query)
	}

	// "Akademický slovník současné češtiny"

	if conf.Services.ASSC.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for ASSC")
			return
		}
		asscActions := assc.NewASSCActions(
			globalCtx,
			&conf.Services.ASSC,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/assc", asscActions.Query)
		log.Info().Msg("Service ASSC enabled")
	}

	// "Slovník spisovného jazyka českého"

	if conf.Services.SSJC.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for SSJC")
			return
		}
		ssjcActions := ssjc.NewSSJCActions(
			globalCtx,
			&conf.Services.SSJC,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/ssjc", ssjcActions.Query)
		log.Info().Msg("Service SSJC enabled")
	}

	// "Příruční slovník jazyka českého"

	if conf.Services.PSJC.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for PSJC")
			return
		}
		psjcActions := psjc.NewPSJCActions(
			globalCtx,
			&conf.Services.PSJC,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/psjc", psjcActions.Query)
		log.Info().Msg("Service PSJC enabled")
	}

	// "Kartotéka lexikálního archivu"

	if conf.Services.KLA.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for KLA")
			return
		}
		klaActions := kla.NewKLAActions(
			globalCtx,
			&conf.Services.KLA,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/kla", klaActions.Query)
		log.Info().Msg("Service KLA enabled")
	}

	// "Neomat"

	if conf.Services.Neomat.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for Neomat")
			return
		}
		neomatActions := neomat.NewNeomatActions(
			globalCtx,
			&conf.Services.Neomat,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/neomat", neomatActions.Query)
		log.Info().Msg("Service Neomat enabled")
	}

	// "Český jazykový atlas"

	if conf.Services.CJA.BaseURL != "" {
		guard, err := tlmtr.New(globalCtx, &conf.Botwatch, conf.Telemetry)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to instantiate guard for CJA")
			return
		}
		cjaActions := cja.NewCJAActions(
			globalCtx,
			&conf.Services.CJA,
			guard,
			conf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET("/service/cja", cjaActions.Query)
		log.Info().Msg("Service CJA enabled")
	}

	// common for KonText & Treq

	servicesDefaults := make(map[string]defaults.DefaultsProvider)
	sessActions := defaults.NewActions(servicesDefaults)

	// KonText (API) proxy

	if conf.Services.Kontext.ExternalURL != "" {
		kontextGuard := sessionmap.New(
			globalCtx,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Kontext.ExternalSessionCookieName,
		)

		var kontextReqCounter chan<- guard.RequestInfo
		if len(conf.Services.Kontext.Limits) > 0 {
			kontextReqCounter = alarm.Register(
				"kontext", conf.Services.Kontext.Alarm, conf.Services.Kontext.Limits)
		}
		kontextActions, err := kontext.NewKontextProxy(
			globalCtx,
			&conf.Services.Kontext,
			&cnc.EnvironConf{
				CNCAuthCookie:     conf.CNCAuth.SessionCookieName,
				AuthTokenEntry:    authTokenEntry,
				ServicePath:       "/service/kontext",
				ServiceName:       "kontext",
				CNCPortalLoginURL: cncPortalLoginURL,
				ReadTimeoutSecs:   conf.ServerReadTimeoutSecs,
			},
			kontextGuard,
			kontextReqCounter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start services")
		}

		apiRoutes.Any("/service/kontext/*path", func(ctx *gin.Context) {
			if ctx.Param("path") == "/login" && ctx.Request.Method == http.MethodPost {
				kontextActions.Login(ctx)

			} else if ctx.Param("path") == "/preflight" {
				kontextActions.Preflight(ctx)

			} else {
				kontextActions.AnyPath(ctx)
			}
		})
		servicesDefaults["kontext"] = kontextActions
		log.Info().Msg("Service Kontext enabled")
	}

	// MQuery proxy

	if conf.Services.MQuery.ExternalURL != "" {
		var mqueryReqCounter chan<- guard.RequestInfo
		if len(conf.Services.MQuery.Limits) > 0 {
			mqueryReqCounter = alarm.Register(
				"mquery", conf.Services.MQuery.Alarm, conf.Services.MQuery.Limits)
		}
		mqueryActions, err := mquery.NewMQueryProxy(
			globalCtx,
			&conf.Services.MQuery,
			&cnc.EnvironConf{
				CNCAuthCookie:     conf.CNCAuth.SessionCookieName,
				AuthTokenEntry:    authTokenEntry,
				ServicePath:       "/service/mquery",
				ServiceName:       "mquery",
				CNCPortalLoginURL: cncPortalLoginURL,
				ReadTimeoutSecs:   conf.ServerReadTimeoutSecs,
			},
			dflt.New(globalCtx, conf.CNCAuth.SessionCookieName),
			mqueryReqCounter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start services")
			return
		}

		apiRoutes.Any("/service/mquery/*path", func(ctx *gin.Context) {
			if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
				mqueryActions.Login(ctx)

			} else if ctx.Param("path") == "/preflight" {
				mqueryActions.Preflight(ctx)

			} else {
				mqueryActions.AnyPath(ctx)
			}
		})
		log.Info().Msg("Service MQuery enabled")
	}

	// MQuery-GPT proxy

	if conf.Services.MQueryGPT.ExternalURL != "" {
		cnca := sessionmap.New(
			globalCtx,
			conf.CNCAuth.SessionCookieName,
			conf.Services.MQueryGPT.ExternalSessionCookieName,
		)

		var mqueryReqCounter chan<- guard.RequestInfo
		if len(conf.Services.MQueryGPT.Limits) > 0 {
			mqueryReqCounter = alarm.Register(
				"mquery-gpt", conf.Services.MQueryGPT.Alarm, conf.Services.MQueryGPT.Limits)
		}
		mqueryActions, err := mquery.NewMQueryProxy(
			globalCtx,
			&conf.Services.MQueryGPT,
			&cnc.EnvironConf{
				CNCAuthCookie:     conf.CNCAuth.SessionCookieName,
				AuthTokenEntry:    authTokenEntry,
				ServicePath:       "/service/mquery-gpt",
				ServiceName:       "mquery-gpt",
				CNCPortalLoginURL: cncPortalLoginURL,
				ReadTimeoutSecs:   conf.ServerReadTimeoutSecs,
			},
			cnca,
			mqueryReqCounter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start services")
			return
		}

		apiRoutes.Any("/service/mquery-gpt/*path", func(ctx *gin.Context) {
			if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
				mqueryActions.Login(ctx)

			} else if ctx.Param("path") == "/preflight" {
				mqueryActions.Preflight(ctx)

			} else {
				mqueryActions.AnyPath(ctx)
			}
		})
		log.Info().Msg("Service MQuery-GPT enabled")
	}

	// Treq (API) proxy

	if conf.Services.Treq.ExternalURL != "" {
		cnca := sessionmap.New(
			globalCtx,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Treq.ExternalSessionCookieName,
		)
		var treqReqCounter chan<- guard.RequestInfo
		if len(conf.Services.Treq.Limits) > 0 {
			treqReqCounter = alarm.Register(
				treq.ServiceName, conf.Services.Treq.Alarm, conf.Services.Treq.Limits)
		}
		treqActions, err := treq.NewTreqProxy(
			globalCtx,
			&conf.Services.Treq,
			conf.CNCAuth.SessionCookieName,
			cnca,
			conf.ServerReadTimeoutSecs,
			treqReqCounter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to initialize Treq proxy")
			return
		}
		apiRoutes.Any("/service/treq/*path", treqActions.AnyPath)
		log.Info().Msg("Service Treq enabled")
	}

	// KWords (API) proxy

	if conf.Services.KWords.ExternalURL != "" {
		client := httpclient.New(
			httpclient.WithFollowRedirects(),
			httpclient.WithInsecureSkipVerify(),
			httpclient.WithIdleConnTimeout(time.Duration(60)*time.Second),
		)
		analyzer := dflt.New(
			globalCtx,
			conf.CNCAuth.SessionCookieName,
		)
		go analyzer.Run()
		internalURL, err := url.Parse(conf.Services.KWords.InternalURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to configure internal URL for KWords")
			return
		}
		externalURL, err := url.Parse(conf.Services.KWords.ExternalURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to configure external URL for KWords")
			return
		}
		coreProxy, err := proxy.NewAPIProxy(conf.Services.KWords.GetCoreConf())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to initialize proxy")
			return
		}

		kwordsActions := proxy.NewPublicAPIProxy(
			coreProxy,
			client,
			analyzer.ExposeAsCounter(),
			analyzer,
			globalCtx.CNCDB,
			proxy.PublicAPIProxyOpts{
				ServiceName:      "kwords",
				InternalURL:      internalURL,
				ExternalURL:      externalURL,
				AuthCookieName:   conf.CNCAuth.SessionCookieName,
				UserIDHeaderName: conf.Services.KWords.TrueUserIDHeader,
				ReadTimeoutSecs:  conf.ServerReadTimeoutSecs,
			},
		)
		apiRoutes.Any("/service/kwords/*path", kwordsActions.AnyPath)
		log.Info().Msg("Service KWords enabled")
	}

	// session tools

	apiRoutes.GET("/defaults/:serviceID/:key", sessActions.Get)

	apiRoutes.POST("/defaults/:serviceID/:key", sessActions.Set)

	// administration/monitoring actions

	telemetryActions := tstorage.NewActions(globalCtx.TelemetryDB)
	apiRoutes.POST("/telemetry", telemetryActions.Store)

	apiRoutes.GET("/delayLogsAnalysis", func(ctx *gin.Context) {
		binWidth, otherLimit := 0.1, 5.0
		var err error

		queryValue := ctx.Request.URL.Query().Get("binwidth")
		if queryValue != "" {
			binWidth, err = strconv.ParseFloat(queryValue, 64)
			if err != nil {
				uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		queryValue = ctx.Request.URL.Query().Get("otherlimit")
		if queryValue != "" {
			otherLimit, err = strconv.ParseFloat(queryValue, 64)
			if err != nil {
				uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		ans, err := globalCtx.TelemetryDB.AnalyzeDelayLog(binWidth, otherLimit)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			uniresp.WriteJSONResponse(ctx.Writer, ans)
		}
	})

	apiRoutes.GET("/bans", func(ctx *gin.Context) {
		duration := time.Duration(24 * time.Hour)
		var err error

		queryValue := ctx.Request.URL.Query().Get("timeAgo")
		if queryValue != "" {
			duration, err = datetime.ParseDuration(queryValue)
			if err != nil {
				uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		ans, err := globalCtx.TelemetryDB.AnalyzeBans(duration)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			uniresp.WriteJSONResponse(ctx.Writer, ans)
		}
	})

	log.Info().Msgf("starting to listen at %s:%d", conf.ServerHost, conf.ServerPort)

	srv := &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", conf.ServerHost, conf.ServerPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	globalCtx.ReportingWriter.LogErrors()

	<-globalCtx.Done()
	// now let's give subsystems some time to save state, clean-up etc.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("HTTP server shutdown error")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := alarm.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("AlarmTicker shutdown error")
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Graceful shutdown completed")
	case <-ctx.Done():
		log.Warn().Msg("Shutdown timed out")
	}
}
