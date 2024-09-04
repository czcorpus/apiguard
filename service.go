// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/alarms"
	"apiguard/config"
	"apiguard/guard"
	"apiguard/guard/dflt"
	"apiguard/guard/sessionmap"
	"apiguard/guard/telemetry"
	"apiguard/proxy"
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
	"apiguard/services/requests"
	"apiguard/services/tstorage"
	userHandlers "apiguard/users/handlers"
	"context"
	"fmt"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const (
	cncPortalLoginURL = "https://www.korpus.cz/login"
	authTokenEntry    = "personal_access_token"
)

func runService(conf *config.Configuration, pgPool *pgxpool.Pool) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, syscall.SIGHUP)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	globalCtx := createGlobalCtx(ctx, conf, pgPool)

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
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	// alarm
	alarm := alarms.NewAlarmTicker(
		globalCtx,
		conf.TimezoneLocation(),
		conf.Mail,
		conf.StatusDataDir,
	)
	go alarm.Run(reloadChan)
	alarm.GoStartMonitoring()

	if !conf.IgnoreStoredState {
		err := alarms.LoadState(alarm)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("Failed to load alarm status from disk. Please use -ignore-stored-state to " +
					"skip the action or remove the problematic file.")
		}
	}

	engine.POST("/alarm/:alarmID/confirmation", alarm.HandleReviewAction)
	engine.GET("/alarm-confirmation", alarm.HandleConfirmationPage)
	engine.GET("/alarm", alarm.HandleReportListAction)
	engine.GET("/alarms/list", alarm.HandleListAction)
	engine.POST("/alarms/clean", alarm.HandleCleanAction)

	// delay stats writer and telemetry analyzer
	delayStats := guard.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	telemetryAnalyzer, err := telemetry.New(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.TimescaleDBWriter,
		delayStats,
		delayStats,
	)
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	// ----------------------

	engine.GET("/service/ping", func(ctx *gin.Context) {
		globalCtx.TimescaleDBWriter.Write(&PingReport{
			DateTime: time.Now(),
			Status:   200,
		})
		uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"ok": true})
	})

	// --------------------

	// "Jazyková příručka ÚJČ"

	if conf.Services.LanguageGuide.BaseURL != "" {
		langGuideActions := lguide.NewLanguageGuideActions(
			globalCtx,
			&conf.Services.LanguageGuide,
			&conf.Botwatch,
			&conf.Telemetry,
			conf.ServerReadTimeoutSecs,
			delayStats,
			telemetryAnalyzer,
		)
		engine.GET("/service/language-guide", langGuideActions.Query)
	}

	// "Akademický slovník současné češtiny"

	if conf.Services.ASSC.BaseURL != "" {
		asscActions := assc.NewASSCActions(
			globalCtx,
			&conf.Services.ASSC,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/assc", asscActions.Query)
		log.Info().Msg("Service ASSC enabled")
	}

	// "Slovník spisovného jazyka českého"

	if conf.Services.SSJC.BaseURL != "" {
		ssjcActions := ssjc.NewSSJCActions(
			globalCtx,
			&conf.Services.SSJC,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/ssjc", ssjcActions.Query)
		log.Info().Msg("Service SSJC enabled")
	}

	// "Příruční slovník jazyka českého"

	if conf.Services.PSJC.BaseURL != "" {
		psjcActions := psjc.NewPSJCActions(
			globalCtx,
			&conf.Services.PSJC,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/psjc", psjcActions.Query)
		log.Info().Msg("Service PSJC enabled")
	}

	// "Kartotéka lexikálního archivu"

	if conf.Services.KLA.BaseURL != "" {
		klaActions := kla.NewKLAActions(
			globalCtx,
			&conf.Services.KLA,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/kla", klaActions.Query)
		log.Info().Msg("Service KLA enabled")
	}

	// "Neomat"

	if conf.Services.Neomat.BaseURL != "" {
		neomatActions := neomat.NewNeomatActions(
			globalCtx,
			&conf.Services.Neomat,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/neomat", neomatActions.Query)
		log.Info().Msg("Service Neomat enabled")
	}

	// "Český jazykový atlas"

	if conf.Services.CJA.BaseURL != "" {
		cjaActions := cja.NewCJAActions(
			globalCtx,
			&conf.Services.CJA,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		engine.GET("/service/cja", cjaActions.Query)
		log.Info().Msg("Service CJA enabled")
	}

	// common for KonText & Treq

	servicesDefaults := make(map[string]defaults.DefaultsProvider)
	sessActions := defaults.NewActions(servicesDefaults)

	// KonText (API) proxy

	if conf.Services.Kontext.ExternalURL != "" {
		kontextGuard := sessionmap.New(
			globalCtx,
			delayStats,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Kontext.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
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

		engine.Any("/service/kontext/*path", func(ctx *gin.Context) {
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
		cnca := sessionmap.New(
			globalCtx,
			delayStats,
			conf.CNCAuth.SessionCookieName,
			conf.Services.MQuery.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)

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
			cnca,
			mqueryReqCounter,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start services")
			return
		}

		engine.Any("/service/mquery/*path", func(ctx *gin.Context) {
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
			delayStats,
			conf.CNCAuth.SessionCookieName,
			conf.Services.MQueryGPT.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
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

		engine.Any("/service/mquery-gpt/*path", func(ctx *gin.Context) {
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
			delayStats,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Treq.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)
		var treqReqCounter chan<- guard.RequestInfo
		if len(conf.Services.Treq.Limits) > 0 {
			treqReqCounter = alarm.Register(treq.ServiceName, conf.Services.Treq.Alarm, conf.Services.Treq.Limits)
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
		engine.Any("/service/treq/*path", treqActions.AnyPath)
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
			globalCtx.CNCDB,
			delayStats,
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
				UserIDHeaderName: conf.Services.KWords.UserIDPassHeader,
				ReadTimeoutSecs:  conf.ServerReadTimeoutSecs,
			},
		)
		engine.Any("/service/kwords/*path", kwordsActions.AnyPath)
		log.Info().Msg("Service KWords enabled")
	}

	// user handling

	usersActions := userHandlers.NewActions(globalCtx.CNCDB, conf.TimezoneLocation())

	engine.GET("/user/:userID/ban", usersActions.BanInfo)

	engine.PUT("/user/:userID/ban", usersActions.SetBan)

	engine.DELETE("/user/:userID/ban", usersActions.DisableBan)

	// session tools

	engine.GET("/defaults/:serviceID/:key", sessActions.Get)

	engine.POST("/defaults/:serviceID/:key", sessActions.Set)

	// administration/monitoring actions

	telemetryActions := tstorage.NewActions(delayStats)
	engine.POST("/telemetry", telemetryActions.Store)

	requestsActions := requests.NewActions(delayStats)
	engine.GET("/requests", requestsActions.List)

	engine.GET("/delayLogsAnalysis", func(ctx *gin.Context) {
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

		ans, err := delayStats.AnalyzeDelayLog(binWidth, otherLimit)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			uniresp.WriteJSONResponse(ctx.Writer, ans)
		}
	})

	engine.GET("/bans", func(ctx *gin.Context) {
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

		ans, err := delayStats.AnalyzeBans(duration)
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

	globalCtx.TimescaleDBWriter.LogErrors()

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
