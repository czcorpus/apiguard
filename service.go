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
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/backend/assc"
	"apiguard/services/backend/cja"
	"apiguard/services/backend/kla"
	"apiguard/services/backend/kontext"
	"apiguard/services/backend/lguide"
	"apiguard/services/backend/neomat"
	"apiguard/services/backend/psjc"
	"apiguard/services/backend/ssjc"
	"apiguard/services/backend/treq"
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

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func notFoundHandler() {

}

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

	router := mux.NewRouter()
	router.Use(coreMiddleware)
	router.MethodNotAllowedHandler = uniresp.NotAllowedHandler{}
	router.NotFoundHandler = uniresp.NotFoundHandler{}

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

	router.HandleFunc(
		"/alarm/{alarmID}/confirmation", alarm.HandleReviewAction).Methods(http.MethodPost)

	router.HandleFunc(
		"/alarm-confirmation", alarm.HandleConfirmationPage).Methods(http.MethodGet)

	router.HandleFunc(
		"/alarm", alarm.HandleReportListAction).Methods(http.MethodGet)

	router.HandleFunc(
		"/alarms/list", alarm.HandleListAction).Methods(http.MethodGet)

	router.HandleFunc(
		"/alarms/clean", alarm.HandleCleanAction).Methods(http.MethodPost)

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

	var cache services.Cache
	if conf.Cache.FileRootPath != "" {
		cache = reqcache.NewFileReqCache(&conf.Cache)
		log.Info().Msgf("using file request cache (path: %s)", conf.Cache.FileRootPath)

	} else if conf.Cache.RedisAddr != "" {
		cache = reqcache.NewRedisReqCache(&conf.Cache)
		log.Info().Msgf("using redis request cache (addr: %s, db: %d)", conf.Cache.RedisAddr, conf.Cache.RedisDB)

	} else {
		cache = reqcache.NewNullCache()
		log.Info().Msg("using NULL cache (path not specified)")
	}

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
			cache,
		)
		router.HandleFunc("/service/language-guide", langGuideActions.Query)
	}

	// "Akademický slovník současné češtiny"

	if conf.Services.ASSC.BaseURL != "" {
		asscActions := assc.NewASSCActions(
			globalCtx,
			&conf.Services.ASSC,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/assc", asscActions.Query)
		log.Info().Msg("Service ASSC enabled")
	}

	// "Slovník spisovného jazyka českého"

	if conf.Services.SSJC.BaseURL != "" {
		ssjcActions := ssjc.NewSSJCActions(
			globalCtx,
			&conf.Services.SSJC,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/ssjc", ssjcActions.Query)
		log.Info().Msg("Service SSJC enabled")
	}

	// "Příruční slovník jazyka českého"

	if conf.Services.PSJC.BaseURL != "" {
		psjcActions := psjc.NewPSJCActions(
			globalCtx,
			&conf.Services.PSJC,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/psjc", psjcActions.Query)
		log.Info().Msg("Service PSJC enabled")
	}

	// "Kartotéka lexikálního archivu"

	if conf.Services.KLA.BaseURL != "" {
		klaActions := kla.NewKLAActions(
			globalCtx,
			&conf.Services.KLA,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/kla", klaActions.Query)
		log.Info().Msg("Service KLA enabled")
	}

	// "Neomat"

	if conf.Services.Neomat.BaseURL != "" {
		neomatActions := neomat.NewNeomatActions(
			globalCtx,
			&conf.Services.Neomat,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/neomat", neomatActions.Query)
		log.Info().Msg("Service Neomat enabled")
	}

	// "Český jazykový atlas"

	if conf.Services.CJA.BaseURL != "" {
		cjaActions := cja.NewCJAActions(
			globalCtx,
			&conf.Services.CJA,
			cache,
			telemetryAnalyzer,
			conf.ServerReadTimeoutSecs,
		)
		router.HandleFunc("/service/cja", cjaActions.Query)
		log.Info().Msg("Service CJA enabled")
	}

	// common for KonText & Treq

	servicesDefaults := make(map[string]defaults.DefaultsProvider)
	sessActions := defaults.NewActions(servicesDefaults)

	// KonText (API) proxy

	if conf.Services.Kontext.ExternalURL != "" {
		cnca := analyzer.NewCNCUserAnalyzer(
			globalCtx.CNCDB,
			conf.TimezoneLocation(),
			userTableProps,
			conf.CNCAuth.SessionCookieName,
			conf.Services.Kontext.ExternalSessionCookieName,
			conf.CNCDB.AnonymousUserID,
		)

		var kontextReqCounter chan<- alarms.RequestInfo
		if len(conf.Services.Kontext.Limits) > 0 {
			kontextReqCounter = alarm.Register(kontext.ServiceName, conf.Services.Kontext.Alarm, conf.Services.Kontext.Limits)
		}
		kontextActions := kontext.NewKontextProxy(
			globalCtx,
			&conf.Services.Kontext,
			conf.CNCAuth.SessionCookieName,
			cnca,
			conf.ServerReadTimeoutSecs,
			globalCtx.CNCDB,
			kontextReqCounter,
			cache,
		)

		router.HandleFunc("/service/kontext/login", kontextActions.Login).Methods(http.MethodPost)
		router.HandleFunc("/service/kontextpreflight", kontextActions.Preflight) // TODO fix terrible URL patch (proxy issue)
		router.PathPrefix("/service/kontext").HandlerFunc(kontextActions.AnyPath)
		servicesDefaults["kontext"] = kontextActions
		log.Info().Msg("Service Kontext enabled")
	}

	// Treq (API) proxy

	if conf.Services.Treq.ExternalURL != "" {
		cnca := analyzer.NewCNCUserAnalyzer(
			globalCtx.CNCDB,
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
			globalCtx.CNCDB,
			treqReqCounter,
			cache,
		)
		router.PathPrefix("/service/treq").HandlerFunc(treqActions.AnyPath)
		log.Info().Msg("Service Treq enabled")
	}

	// user handling

	usersActions := users.NewActions(&users.Conf{}, globalCtx.CNCDB, conf.TimezoneLocation())

	router.HandleFunc("/user/{userID}/ban", usersActions.BanInfo).Methods(http.MethodGet)

	router.HandleFunc("/user/{userID}/ban", usersActions.SetBan).Methods(http.MethodPut)

	router.HandleFunc("/user/{userID}/ban", usersActions.DisableBan).Methods(http.MethodDelete)

	// session tools

	router.HandleFunc("/defaults/{serviceID}/{key}", sessActions.Get).Methods(http.MethodGet)

	router.HandleFunc("/defaults/{serviceID}/{key}", sessActions.Set).Methods(http.MethodPost)

	// administration/monitoring actions

	telemetryActions := tstorage.NewActions(delayStats)
	router.HandleFunc("/telemetry", telemetryActions.Store).Methods(http.MethodPost)

	requestsActions := requests.NewActions(delayStats)
	router.HandleFunc("/requests", requestsActions.List)

	router.HandleFunc("/delayLogsAnalysis", func(w http.ResponseWriter, req *http.Request) {
		binWidth, otherLimit := 0.1, 5.0
		var err error

		queryValue := req.URL.Query().Get("binwidth")
		if queryValue != "" {
			binWidth, err = strconv.ParseFloat(queryValue, 64)
			if err != nil {
				uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		queryValue = req.URL.Query().Get("otherlimit")
		if queryValue != "" {
			otherLimit, err = strconv.ParseFloat(queryValue, 64)
			if err != nil {
				uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		ans, err := delayStats.AnalyzeDelayLog(binWidth, otherLimit)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				w, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			uniresp.WriteJSONResponse(w, ans)
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
		Handler:      router,
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
