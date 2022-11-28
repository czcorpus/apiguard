// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"context"
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

	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/config"
	"apiguard/logging"
	"apiguard/reqcache"
	"apiguard/services"
	"apiguard/services/assc"
	"apiguard/services/cja"
	"apiguard/services/kla"
	"apiguard/services/kontext"
	kontextDb "apiguard/services/kontext/db"
	"apiguard/services/lguide"
	"apiguard/services/neomat"
	"apiguard/services/psjc"
	"apiguard/services/requests"
	"apiguard/services/ssjc"
	"apiguard/services/tstorage"
	"apiguard/storage"
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
)

type CmdOptions struct {
	Host             string
	Port             int
	ReadTimeoutSecs  int
	WriteTimeoutSecs int
	LogPath          string
	MaxAgeDays       int
	BanSecs          int
}

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func setupLog(path string) {
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

func initStorage(conf *config.Configuration) *storage.MySQLAdapter {
	db, err := storage.NewMySQLAdapter(
		conf.Storage.Host,
		conf.Storage.User,
		conf.Storage.Password,
		conf.Storage.Database,
	)
	if err != nil {
		log.Fatal().Msgf("FATAL: failed to connect to a storage database - %s", err)
	}
	return db
}

func runService(conf *config.Configuration) {
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)

	db := initStorage(conf)

	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		&conf.Monitoring,
		db,
		db,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	var cache services.Cache
	if conf.Cache.RootPath != "" {
		cache = reqcache.NewReqCache(&conf.Cache)
		log.Info().Msgf("using request cache (path: %s)", conf.Cache.RootPath)

	} else {
		cache = reqcache.NewNullCache()
		log.Info().Msg("using NULL cache (path not specified)")
	}

	// "Jazyková příručka ÚJČ"

	langGuideActions := lguide.NewLanguageGuideActions(
		&conf.Services.LanguageGuide,
		&conf.Botwatch,
		&conf.Telemetry,
		conf.ServerReadTimeoutSecs,
		db,
		telemetryAnalyzer,
		cache,
	)
	router.HandleFunc("/service/language-guide", langGuideActions.Query)

	// "Akademický slovník současné češtiny"

	asscActions := assc.NewASSCActions(
		&conf.Services.ASSC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/assc", asscActions.Query)

	// "Slovník spisovného jazyka českého"

	ssjcActions := ssjc.NewSSJCActions(
		&conf.Services.SSJC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/ssjc", ssjcActions.Query)

	// "Příruční slovník jazyka českého"

	psjcActions := psjc.NewPSJCActions(
		&conf.Services.PSJC,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/psjc", psjcActions.Query)

	// "Kartotéka lexikálního archivu"

	klaActions := kla.NewKLAActions(
		&conf.Services.KLA,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/kla", klaActions.Query)

	// "Neomat"

	neomatActions := neomat.NewNeomatActions(
		&conf.Services.Neomat,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/neomat", neomatActions.Query)

	// "Český jazykový atlas"

	cjaActions := cja.NewCJAActions(
		&conf.Services.CJA,
		cache,
		telemetryAnalyzer,
		conf.ServerReadTimeoutSecs,
	)
	router.HandleFunc("/service/cja", cjaActions.Query)

	// KonText (API) proxy

	log.Info().Msgf("CNC SQL database: %s", conf.CNCDB.Host)

	cncDB, err := cncdb.OpenDB(&conf.CNCDB)
	if err != nil {
		log.Fatal().Err(err)
	}
	userTableName := cncdb.DfltUsersTableName
	if conf.CNCDB.OverrideUsersTableName != "" {
		userTableName = conf.CNCDB.OverrideUsersTableName
	}
	kua := kontextDb.NewKonTextUsersAnalyzer(
		cncDB,
		conf.TimezoneLocation(),
		userTableName,
		conf.Services.Kontext.SessionCookieName,
		conf.CNCDB.AnonymousUserID,
	)
	kontextActions := kontext.NewKontextProxy(
		&conf.Services.Kontext,
		kua,
		conf.ServerReadTimeoutSecs,
		cncDB,
	)
	router.PathPrefix("/service/kontext").HandlerFunc(kontextActions.AnyPath)

	// user handling

	usersActions := users.NewActions(&users.Conf{}, cncDB, conf.TimezoneLocation())

	router.HandleFunc("/user/{userID}/ban", usersActions.BanInfo).Methods(http.MethodGet)

	router.HandleFunc("/user/{userID}/ban", usersActions.SetBan).Methods(http.MethodPut)

	router.HandleFunc("/user/{userID}/ban", usersActions.DisableBan).Methods(http.MethodDelete)

	// administration/monitoring actions

	telemetryActions := tstorage.NewActions(db)
	router.HandleFunc("/telemetry", telemetryActions.Store).Methods(http.MethodPost)

	requestsActions := requests.NewActions(db)
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

		ans, err := db.AnalyzeDelayLog(binWidth, otherLimit)
		if err != nil {
			services.WriteJSONErrorResponse(
				w, services.NewActionError(err.Error()), http.StatusInternalServerError)
		} else {
			services.WriteJSONResponse(w, ans)
		}
	})

	go func() {
		evt := <-syscallChan
		exitEvent <- evt
		close(exitEvent)
	}()

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

func runCleanup(conf *config.Configuration) {
	log.Info().Msg("running cleanup procedure")
	db := initStorage(conf)
	ans := db.CleanOldData(conf.CleanupMaxAgeDays)
	if ans.Error != nil {
		log.Fatal().Err(ans.Error).Msg("failed to cleanup old records")
	}
	status, err := json.Marshal(ans)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to provide cleanup summary")
	}
	log.Info().Msgf("finished old data cleanup: %s", string(status))
}

func runStatus(conf *config.Configuration, storage *storage.MySQLAdapter, ident string) {

	ip := net.ParseIP(ident)
	var sessionID string
	if ip == nil {
		var err error
		log.Info().Msgf("assuming %s is a session ID", ident)
		sessionID = logging.NormalizeSessionID(ident)
		ip, err = storage.GetSessionIP(sessionID)
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
		&conf.Monitoring,
		storage,
		storage,
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
		ipStats, err := storage.LoadIPStats(ip.String(), conf.Telemetry.MaxAgeSecsRelevant)
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

func runLearn(conf *config.Configuration, storage *storage.MySQLAdapter) {

	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		&conf.Monitoring,
		storage,
		storage,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	err = telemetryAnalyzer.Learn()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "read-timeout", 0, "Server read timeout in seconds")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "write-timeout", 0, "Server write timeout in seconds")
	flag.StringVar(&cmdOpts.LogPath, "log-path", "", "A file to log to (if empty then stderr is used)")
	flag.IntVar(&cmdOpts.MaxAgeDays, "max-age-days", 0, "When cleaning old records, this specifies the oldes records (in days) to keep in database.")
	flag.IntVar(&cmdOpts.BanSecs, "ban-secs", 0, "Number of seconds to ban an IP address")
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"apiguard - CNC API protection and response data polisher"+
				"\n\nUsage:"+
				"\n\t%s [options] start [conf.json]"+
				"\n\t%s [options] cleanup [conf.json]"+
				"\n\t%s [options] ban [ip address] [conf.json]"+
				"\n\t%s [options] unban [ip address] [conf.json]"+
				"\n\t%s [options] status [session id / IP address] [conf.json]"+
				"\n\t%s [options] learn [conf.json]"+
				"\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]),
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
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts, setupLog)
		log.Info().
			Str("version", versionInfo.Version).
			Str("buildDate", versionInfo.BuildDate).
			Str("last commit", versionInfo.GitCommit).
			Msg("Starting CNC APIGuard")
		runService(conf)
	case "cleanup":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts, setupLog)
		runCleanup(conf)
	case "ban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts, setupLog)
		db := initStorage(conf)
		if err := db.InsertBan(net.ParseIP(flag.Arg(1)), conf.BanTTLSecs); err != nil {
			log.Fatal().Err(err).Msg("")
		}
	case "unban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts, setupLog)
		db := initStorage(conf)
		if err := db.RemoveBan(net.ParseIP(flag.Arg(1))); err != nil {
			log.Fatal().Err(err).Msg("")
		}
	case "status":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts, setupLog)
		db := initStorage(conf)
		runStatus(conf, db, flag.Arg(1))
	case "learn":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts, setupLog)
		db := initStorage(conf)
		runLearn(conf, db)
	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
