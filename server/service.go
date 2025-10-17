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

package server

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/czcorpus/apiguard/cnc"
	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/monitoring"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/proxy/cache"
	"github.com/czcorpus/apiguard/proxy/cache/file"
	"github.com/czcorpus/apiguard/proxy/cache/null"
	"github.com/czcorpus/apiguard/proxy/cache/redis"
	"github.com/czcorpus/apiguard/reporting"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/czcorpus/apiguard/tstorage"
	"github.com/czcorpus/apiguard/wagstream"
	"github.com/czcorpus/hltscl"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	_ "github.com/czcorpus/apiguard/services/backend/frodo"
	_ "github.com/czcorpus/apiguard/services/backend/gunstick"
	_ "github.com/czcorpus/apiguard/services/backend/hex"
	_ "github.com/czcorpus/apiguard/services/backend/kontext"
	_ "github.com/czcorpus/apiguard/services/backend/kwords"
	_ "github.com/czcorpus/apiguard/services/backend/mquery"
	_ "github.com/czcorpus/apiguard/services/backend/scollex"
	_ "github.com/czcorpus/apiguard/services/backend/treq"
	_ "github.com/czcorpus/apiguard/services/backend/wss"
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
			globalCtx.BackendLoggers.Get("ping").Log(
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

	srvfactory.InitServices(
		globalCtx,
		engine,
		apiRoutes,
		conf,
		alarm,
	)

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

func createPGPool(conf hltscl.PgConf) *pgxpool.Pool {
	conn, err := hltscl.CreatePool(conf)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	return conn
}

func CreateTDBWriter(
	ctx context.Context, conf *reporting.Conf, loc *time.Location) (ans reporting.ReportingWriter) {
	if conf != nil {
		pgPool := createPGPool(conf.DB)
		ans = reporting.NewReportingWriter(pgPool, loc, ctx)

	} else {
		ans = &reporting.NullWriter{}
	}
	return
}

func openCNCDatabase(conf *cnc.Conf) *sql.DB {
	if conf == nil {
		log.Info().Msgf("No CNC database configured")
		return nil
	}
	cncDB, err := cnc.OpenDB(conf)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().
		Str("host", conf.Host).
		Str("name", conf.Name).
		Str("user", conf.User).
		Msg("Connected to CNC's SQL database")
	return cncDB
}

func CreateGlobalCtx(
	ctx context.Context,
	conf *config.Configuration,
	tDBWriter reporting.ReportingWriter,
) (*globctx.Context, error) {
	ans := globctx.NewGlobalContext(ctx)

	tDBWriter.AddTableWriter(reporting.AlarmMonitoringTable)
	tDBWriter.AddTableWriter(reporting.BackendMonitoringTable)
	tDBWriter.AddTableWriter(reporting.ProxyMonitoringTable)
	tDBWriter.AddTableWriter(reporting.TelemetryMonitoringTable)

	cncdb := openCNCDatabase(conf.CNCDB)

	var cacheBackend cache.Cache
	if conf.Cache.FileRootPath != "" {
		cacheBackend = file.New(conf.Cache)
		log.Info().Msgf("using file response cache (path: %s)", conf.Cache.FileRootPath)
		log.Warn().Msg("caching respects the Cache-Control header")

	} else if conf.Cache.RedisAddr != "" {
		cacheBackend = redis.New(ctx, conf.Cache)
		log.Info().Msgf("using redis response cache (addr: %s, db: %d)", conf.Cache.RedisAddr, conf.Cache.RedisDB)
		log.Warn().Msg("caching respects the Cache-Control header")

	} else {
		cacheBackend = null.New()
		log.Warn().Msg("using NULL cache (neither fs path nor Redis props are specified)")
	}

	ans.TimezoneLocation = conf.TimezoneLocation()
	ans.ReportingWriter = tDBWriter
	ans.BackendLoggers = make(map[string]*globctx.BackendLogger)
	for i, backendConf := range conf.Services {
		var err error
		serviceKey := fmt.Sprintf("%d/%s", i, backendConf.Type)
		ans.BackendLoggers[serviceKey], err = globctx.NewBackendLogger(
			tDBWriter,
			backendConf.LogPath,
			fmt.Sprintf("/service/%s", serviceKey),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create global ctx: %w", err)
		}
	}
	var err error
	ans.BackendLoggers["default"], err = globctx.NewBackendLogger(tDBWriter, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create global ctx: %w", err)
	}
	ans.CNCDB = cncdb
	ans.Cache = cacheBackend
	if conf.CNCDB == nil {
		ans.AnonymousUserIDs = common.AnonymousUsers{}
	} else {
		ans.AnonymousUserIDs = conf.CNCDB.AnonymousUserIDs
	}
	
	// delay stats writer and telemetry analyzer
	ans.TelemetryDB = tstorage.Open(ans.CNCDB, ans.TimezoneLocation)
	return ans, nil
}

func RunService(conf *config.Configuration) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, syscall.SIGHUP)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	tDBWriter := CreateTDBWriter(ctx, conf.Reporting, conf.TimezoneLocation())
	globalCtx, err := CreateGlobalCtx(ctx, conf, tDBWriter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %s", err)
		os.Exit(1)
	}
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
		engine = initWagStreamingEngine(actionsHandler)
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
