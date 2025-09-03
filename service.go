// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/common"
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/tstorage"
	"apiguard/wagstream"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	cncPortalLoginURL = "https://www.korpus.cz/login"
	authTokenEntry    = "personal_access_token"
)

func initProxyEngine(
	conf *config.Configuration,
	globalCtx *globctx.Context,
	alarm *monitoring.AlarmTicker,
	skipIPFilter bool,
) http.Handler {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	apiRoutes := engine.Group("/")
	apiRoutes.Use(uniresp.AlwaysJSONContentType())
	apiRoutes.Use(func(ctx *gin.Context) {
		if !conf.IPAllowedForAPI(net.ParseIP(ctx.ClientIP())) && !skipIPFilter {
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

	if err := InitServices(
		globalCtx,
		engine,
		apiRoutes,
		conf,
		alarm,
	); err != nil {
		log.Fatal().Err(err).Msg("failed to start APIGuard")
		return nil
	}

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

	return engine
}

func initWagStreamingEngine(
	conf *config.Configuration,
	actionHandler *wagstream.Actions,
) http.Handler {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	engine.PUT("/wag/stream", actionHandler.CreateStream)
	engine.GET("/wag/stream/:id", actionHandler.StartStream)
	engine.GET("/wag/tileconf/:id", actionHandler.TileConf)

	return engine
}

func runService(conf *config.Configuration) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, syscall.SIGHUP)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	tDBWriter := createTDBWriter(ctx, conf.Reporting, conf.TimezoneLocation())
	globalCtx := createGlobalCtx(ctx, conf, tDBWriter)
	reloadChan := make(chan bool)

	// alarm
	alarm := monitoring.NewAlarmTicker(
		globalCtx,
		conf.TimezoneLocation(),
		conf.Mail,
		conf.PublicRoutesURL,
		conf.Monitoring,
	)
	go alarm.Run(reloadChan)

	go func() {
		for evt := range syscallChan {
			log.Warn().Str("signalName", evt.String()).Msg("received OS signal")
			if evt == syscall.SIGHUP {
				reloadChan <- true
			}
		}
		close(reloadChan)
	}()

	var engine http.Handler

	switch conf.OperationMode {
	case config.OperationModeProxy:
		engine = initProxyEngine(conf, globalCtx, alarm, false)
		log.Info().Msg("running in the PROXY mode")
	case config.OperationModeStreaming:
		apiEngine := initProxyEngine(conf, globalCtx, alarm, true)
		// note that in streaming mode, caching for individual backend
		// handlers is set to Null cache and only possible caching
		// is centralized here

		actionsHandler, err := wagstream.NewActions(ctx, apiEngine, conf.WagTilesConfDir)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start")
			return
		}
		engine = initWagStreamingEngine(conf, actionsHandler)
		log.Info().Msg("running in the STREAMING mode")
	default:
		engine = nil
		log.Fatal().Err(conf.OperationMode.Validate()).Msg("unsupported operation mode")
		return
	}

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
