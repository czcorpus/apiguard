// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"apiguard/alarms"
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/cncdb/analyzer"
	"apiguard/common"
	"apiguard/config"
	"apiguard/ctx"
	"apiguard/monitoring/influx"
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
	"apiguard/services/logging"
	"apiguard/services/requests"
	"apiguard/services/tstorage"
	"apiguard/users"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version     string
	buildDate   string
	gitCommit   string
	versionInfo = services.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}
	levelMapping = map[string]zerolog.Level{
		"debug":   zerolog.DebugLevel,
		"info":    zerolog.InfoLevel,
		"warning": zerolog.WarnLevel,
		"warn":    zerolog.WarnLevel,
		"error":   zerolog.ErrorLevel,
	}
)

type CmdOptions struct {
	Host              string
	Port              int
	ReadTimeoutSecs   int
	WriteTimeoutSecs  int
	LogPath           string
	LogLevel          string
	MaxAgeDays        int
	BanSecs           int
	IgnoreStoredState bool
}

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Add("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

func setupLog(path, level string) {
	lev, ok := levelMapping[level]
	if !ok {
		log.Fatal().Msgf("invalid logging level: %s", level)
	}
	zerolog.SetGlobalLevel(lev)
	if path != "" {
		logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal().Msgf("Failed to initialize log. File: %s", path)
		}
		log.Logger = log.Output(logf)

	} else {
		log.Logger = log.Output(
			zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: time.RFC3339,
			},
		)
	}
}

func openCNCDatabase(conf *cncdb.Conf) *sql.DB {
	cncDB, err := cncdb.OpenDB(conf)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().
		Str("host", conf.Host).
		Str("name", conf.Name).
		Str("user", conf.User).
		Msgf("Connected to CNC's SQL database")
	return cncDB
}

func openInfluxDB(conf *influx.ConnectionConf) *influx.InfluxDBAdapter {
	db := influx.ConnectAPI(conf)
	db.OnError(func(err error) {
		log.Err(err).Msg("Failed to write measurement to InfluxDB")
	})
	log.Info().
		Str("host", conf.Server).
		Str("organization", conf.Organization).
		Str("bucket", conf.Bucket).
		Msgf("Connected to InfluxDB (v2)")
	return db
}

func preExit(alarm *alarms.AlarmTicker) {
	err := alarm.SaveAttributes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to save alarm attributes")
	}
}

func runService(
	globalCtx *ctx.GlobalContext,
	conf *config.Configuration,
	userTableProps cncdb.UserTableProps,
) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	// alarm
	alarm := alarms.NewAlarmTicker(
		globalCtx.CNCDB,
		conf.TimezoneLocation(),
		conf.Mail,
		userTableProps,
		conf.StatusDataDir,
	)

	if !conf.IgnoreStoredState {
		err := alarm.LoadAttributes()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load alarm status from disk. Please use -ignore-stored-state to skip the action.")
		}
	}

	router.HandleFunc(
		"/alarm/{alarmID}/confirmation", alarm.HandleReviewAction).Methods(http.MethodPost)

	router.HandleFunc(
		"/alarm-confirmation", alarm.HandleConfirmationPage).Methods(http.MethodGet)

	router.HandleFunc(
		"/alarm", alarm.HandleReportListAction).Methods(http.MethodGet)

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

	// "Akademický slovník současné češtiny"

	asscActions := assc.NewASSCActions(
		globalCtx,
		&conf.Services.ASSC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/assc", asscActions.Query)

	// "Slovník spisovného jazyka českého"

	ssjcActions := ssjc.NewSSJCActions(
		globalCtx,
		&conf.Services.SSJC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/ssjc", ssjcActions.Query)

	// "Příruční slovník jazyka českého"

	psjcActions := psjc.NewPSJCActions(
		globalCtx,
		&conf.Services.PSJC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/psjc", psjcActions.Query)

	// "Kartotéka lexikálního archivu"

	klaActions := kla.NewKLAActions(
		globalCtx,
		&conf.Services.KLA,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/kla", klaActions.Query)

	// "Neomat"

	neomatActions := neomat.NewNeomatActions(
		globalCtx,
		&conf.Services.Neomat,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/neomat", neomatActions.Query)

	// "Český jazykový atlas"

	cjaActions := cja.NewCJAActions(
		globalCtx,
		&conf.Services.CJA,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/cja", cjaActions.Query)

	// KonText (API) proxy

	cnca := analyzer.NewCNCUserAnalyzer(
		globalCtx.CNCDB,
		conf.TimezoneLocation(),
		userTableProps,
		conf.CNCAuth.SessionCookieName,
		conf.CNCDB.AnonymousUserID,
	)

	var kontextReqCounter chan<- alarms.RequestInfo
	if len(conf.Services.Kontext.Limits) > 0 {
		kontextReqCounter = alarm.Register(kontext.ServiceName, conf.Services.Kontext.Alarm, conf.Services.Kontext.Limits)
	}
	kontextActions := kontext.NewKontextProxy(
		globalCtx,
		&conf.Services.Kontext,
		cnca,
		conf.ServerReadTimeoutSecs,
		globalCtx.CNCDB,
		kontextReqCounter,
		cache,
	)
	router.PathPrefix("/service/kontext").HandlerFunc(kontextActions.AnyPath)

	// Treq (API) proxy

	var treqReqCounter chan<- alarms.RequestInfo
	if len(conf.Services.Treq.Limits) > 0 {
		treqReqCounter = alarm.Register(treq.ServiceName, conf.Services.Treq.Alarm, conf.Services.Treq.Limits)
	}
	treqActions := treq.NewTreqProxy(
		globalCtx,
		&conf.Services.Treq,
		cnca,
		conf.ServerReadTimeoutSecs,
		globalCtx.CNCDB,
		treqReqCounter,
		cache,
	)
	router.PathPrefix("/service/treq").HandlerFunc(treqActions.AnyPath)

	// user handling

	usersActions := users.NewActions(&users.Conf{}, globalCtx.CNCDB, conf.TimezoneLocation())

	router.HandleFunc("/user/{userID}/ban", usersActions.BanInfo).Methods(http.MethodGet)

	router.HandleFunc("/user/{userID}/ban", usersActions.SetBan).Methods(http.MethodPut)

	router.HandleFunc("/user/{userID}/ban", usersActions.DisableBan).Methods(http.MethodDelete)

	// session tools

	sessActions := defaults.NewActions(
		map[string]defaults.DefaultsProvider{
			"kontext": kontextActions,
		},
	)

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
				services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		queryValue = req.URL.Query().Get("otherlimit")
		if queryValue != "" {
			otherLimit, err = strconv.ParseFloat(queryValue, 64)
			if err != nil {
				services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), http.StatusBadRequest)
				return
			}
		}

		ans, err := delayStats.AnalyzeDelayLog(binWidth, otherLimit)
		if err != nil {
			services.WriteJSONErrorResponse(
				w, services.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			services.WriteJSONResponse(w, ans)
		}
	})

	go func() {
		evt := <-syscallChan
		preExit(alarm)
		exitEvent <- evt
		close(exitEvent)
	}()

	go alarm.Run(syscallChan)

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

func runCleanup(db *sql.DB, conf *config.Configuration) {
	log.Info().Msg("running cleanup procedure")
	delayLog := cncdb.NewDelayStats(db, conf.TimezoneLocation())
	ans := delayLog.CleanOldData(conf.CleanupMaxAgeDays)
	if ans.Error != nil {
		log.Fatal().Err(ans.Error).Msg("failed to cleanup old records")
	}
	status, err := json.Marshal(ans)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to provide cleanup summary")
	}
	log.Info().Msgf("finished old data cleanup: %s", string(status))
}

func runStatus(globalCtx ctx.GlobalContext, conf *config.Configuration, ident string) {
	delayLog := cncdb.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	ip := net.ParseIP(ident)
	var sessionID string
	if ip == nil {
		var err error
		log.Info().Msgf("assuming %s is a session ID", ident)
		sessionID = logging.NormalizeSessionID(ident)
		ip, err = delayLog.GetSessionIP(sessionID)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		if ip == nil {
			log.Fatal().Msgf("no IP address found for session %s", sessionID)
		}
	}

	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.InfluxDB,
		delayLog,
		delayLog,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	fakeReq, err := http.NewRequest("POST", "", nil)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if sessionID != "" {
		fakeReq.AddCookie(&http.Cookie{
			Name:  logging.WaGSessionName,
			Value: sessionID,
		})
	}
	fakeReq.RemoteAddr = ip.String()

	if sessionID != "" {
		delay, err := telemetryAnalyzer.CalcDelay(fakeReq)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		botScore, err := telemetryAnalyzer.BotScore(fakeReq)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		fmt.Printf(
			"\nSession: %s"+
				"\nbot score: %01.2f"+
				"\nreq. delay: %v"+
				"\n",
			sessionID, botScore, delay,
		)

	} else {
		ipStats, err := delayLog.LoadIPStats(ip.String(), conf.Telemetry.MaxAgeSecsRelevant)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		d := time.Now().Add(-time.Duration(conf.Botwatch.WatchedTimeWindowSecs) * time.Second)
		fmt.Println(" ", d)
		fmt.Printf(
			"\nShowing stats starting from: %s"+
				"\nIP: %s"+
				"\nNumber of requests: %d"+
				"\nRequests stdev: %01.3f"+
				"\n",
			d, ip.String(), ipStats.Count, ipStats.Stdev(),
		)
	}
}

func runLearn(globalCtx ctx.GlobalContext, conf *config.Configuration) {
	delayLog := cncdb.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.InfluxDB,
		delayLog,
		delayLog,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	err = telemetryAnalyzer.Learn()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func createGlobalCtx(conf *config.Configuration) ctx.GlobalContext {
	influxDB := openInfluxDB(&conf.Monitoring)
	return ctx.GlobalContext{
		TimezoneLocation: conf.TimezoneLocation(),
		InfluxDB:         influxDB,
		BackendLogger:    ctx.NewBackendLogger(influxDB, conf.TimezoneLocation()),
		CNCDB:            openCNCDatabase(&conf.CNCDB),
	}
}

func init() {
	gob.Register(&services.SimpleResponse{})
	gob.Register(&services.ProxiedResponse{})
}

func main() {
	rand.Seed(time.Now().Unix())
	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "read-timeout", 0, "Server read timeout in seconds")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "write-timeout", 0, "Server write timeout in seconds")
	flag.StringVar(&cmdOpts.LogPath, "log-path", "", "A file to log to (if empty then stderr is used)")
	flag.StringVar(&cmdOpts.LogLevel, "log-level", "", "A log level (debug, info, warn/warning, error)")
	flag.IntVar(&cmdOpts.MaxAgeDays, "max-age-days", 0, "When cleaning old records, this specifies the oldes records (in days) to keep in database.")
	flag.IntVar(&cmdOpts.BanSecs, "ban-secs", 0, "Number of seconds to ban an IP address")
	flag.BoolVar(&cmdOpts.IgnoreStoredState, "ignore-stored-state", false, "If used then no alarm state will be loaded from a configured location. This is usefull e.g. in case of an application configuration change.")

	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"apiguard - CNC API protection and response data polishing"+
				"\n\nUsage:"+
				"\n\t%s [options] start [conf.json]"+
				"\n\t%s [options] cleanup [conf.json]"+
				"\n\t%s [options] ipban [ip address] [conf.json]"+
				"\n\t%s [options] ipunban [ip address] [conf.json]"+
				"\n\t%s [options] userban [user ID] [conf.json]"+
				"\n\t%s [options] userunban [user ID] [conf.json]"+
				"\n\t%s [options] status [session id / IP address] [conf.json]"+
				"\n\t%s [options] learn [conf.json]"+
				"\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	action := flag.Arg(0)

	switch action {
	case "version":
		fmt.Printf("CNC APIGuard %s\nbuild date: %s\nlast commit: %s\n",
			versionInfo.Version, versionInfo.BuildDate, versionInfo.GitCommit)
		return
	case "start":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts)
		globalCtx := createGlobalCtx(conf)
		log.Info().
			Str("version", versionInfo.Version).
			Str("buildDate", versionInfo.BuildDate).
			Str("last commit", versionInfo.GitCommit).
			Msg("Starting CNC APIGuard")
		userTableProps := conf.CNCDB.ApplyOverrides()
		runService(&globalCtx, conf, userTableProps)
	case "cleanup":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		runCleanup(db, conf)
	case "ipban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		delayLog := cncdb.NewDelayStats(db, conf.TimezoneLocation())
		if err := delayLog.InsertIPBan(net.ParseIP(flag.Arg(1)), conf.IPBanTTLSecs); err != nil {
			log.Fatal().Err(err).Send()
		}
	case "ipunban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		delayLog := cncdb.NewDelayStats(db, conf.TimezoneLocation())
		if err := delayLog.RemoveIPBan(net.ParseIP(flag.Arg(1))); err != nil {
			log.Fatal().Err(err).Send()
		}
	case "userban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		now := time.Now().In(conf.TimezoneLocation())
		userID, err := common.Str2UserID(flag.Arg(1))
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		banHours := 24
		_, err = cncdb.BanUser(
			db, conf.TimezoneLocation(), userID, nil, now, now.Add(time.Duration(banHours)*time.Hour))
		if err != nil {
			log.Error().Err(err).Msg("Failed to ban user")

		} else {
			log.Info().Msgf("Banned user %d for %d hours", userID, banHours)
		}
	case "userunban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		userID, err := strconv.Atoi(flag.Arg(1))
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		_, err = cncdb.UnbanUser(db, conf.TimezoneLocation(), userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to unban user")

		} else {
			log.Info().Msgf("Unbanned user %d", userID)
		}
	case "status":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		globalCtx := createGlobalCtx(conf)
		runStatus(globalCtx, conf, flag.Arg(1))
	case "learn":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts)
		globalCtx := createGlobalCtx(conf)
		runLearn(globalCtx, conf)
	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
