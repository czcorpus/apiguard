// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/alarms"
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/cncdb/analyzer"
	"apiguard/config"
	"apiguard/ctx"
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
	"apiguard/users"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	cncPortalLoginURL = "https://www.korpus.cz/login"
	authTokenEntry    = "personal_access_token"
)

func runService(
	globalCtx *ctx.GlobalContext,
	conf *config.Configuration,
	userTableProps cncdb.UserTableProps,
) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	signal.Notify(syscallChan, syscall.SIGINT)
	signal.Notify(syscallChan, syscall.SIGHUP)
	exitEvent := make(chan os.Signal)

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
		userTableProps,
		conf.StatusDataDir,
	)
	if conf.Monitoring.IsConfigured() {
		alarm.GoStartMonitoring()
	}

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

	// telemetry analyzer
	delayStats := cncdb.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.InfluxDB,
		delayStats,
		delayStats,
	)
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	// ----------------------

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
		cnca := analyzer.NewCNCUserAnalyzer(
			globalCtx.CNCDB,
			delayStats,
			conf.TimezoneLocation(),
			userTableProps,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Kontext.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)

		var kontextReqCounter chan<- alarms.RequestInfo
		if len(conf.Services.Kontext.Limits) > 0 {
			kontextReqCounter = alarm.Register(
				"kontext", conf.Services.Kontext.Alarm, conf.Services.Kontext.Limits)
		}
		kontextActions := kontext.NewKontextProxy(
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
			cnca,
			kontextReqCounter,
		)

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
		cnca := analyzer.NewCNCUserAnalyzer(
			globalCtx.CNCDB,
			delayStats,
			conf.TimezoneLocation(),
			userTableProps,
			conf.CNCAuth.SessionCookieName,
			conf.Services.MQuery.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)

		var mqueryReqCounter chan<- alarms.RequestInfo
		if len(conf.Services.MQuery.Limits) > 0 {
			mqueryReqCounter = alarm.Register(
				"mquery", conf.Services.MQuery.Alarm, conf.Services.MQuery.Limits)
		}
		mqueryActions := mquery.NewMQueryProxy(
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

		engine.GET("/service/mquery/preflight", mqueryActions.Preflight) // TODO fix terrible URL patch (proxy issue)
		engine.Any("/service/mquery/*path", func(ctx *gin.Context) {
			if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
				mqueryActions.Login(ctx)
			} else {
				mqueryActions.AnyPath(ctx)
			}
		})
		log.Info().Msg("Service MQuery enabled")
	}

	// Treq (API) proxy

	if conf.Services.Treq.ExternalURL != "" {
		cnca := analyzer.NewCNCUserAnalyzer(
			globalCtx.CNCDB,
			delayStats,
			conf.TimezoneLocation(),
			userTableProps,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Treq.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)
		var treqReqCounter chan<- alarms.RequestInfo
		if len(conf.Services.Treq.Limits) > 0 {
			treqReqCounter = alarm.Register(treq.ServiceName, conf.Services.Treq.Alarm, conf.Services.Treq.Limits)
		}
		treqActions := treq.NewTreqProxy(
			globalCtx,
			&conf.Services.Treq,
			conf.CNCAuth.SessionCookieName,
			cnca,
			conf.ServerReadTimeoutSecs,
			treqReqCounter,
		)
		engine.Any("/service/treq/*path", treqActions.AnyPath)
		log.Info().Msg("Service Treq enabled")
	}

	// user handling

	usersActions := users.NewActions(&users.Conf{}, globalCtx.CNCDB, conf.TimezoneLocation())

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

	alarmSyscallChan := make(chan os.Signal, 1)

	go func() {
		for evt := range syscallChan {
			log.Warn().Str("signalName", evt.String()).Msg("received OS signal")
			if evt == syscall.SIGTERM || evt == syscall.SIGINT {
				preExit(alarm)
				exitEvent <- evt
			}
			alarmSyscallChan <- evt
		}
		close(exitEvent)
	}()

	go alarm.Run(alarmSyscallChan)

	log.Info().Msgf("starting to listen at %s:%d", conf.ServerHost, conf.ServerPort)

	srv := &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", conf.ServerHost, conf.ServerPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		syscallChan <- syscall.SIGTERM
	}()

	<-exitEvent
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Shutdown request error")
	}
}
