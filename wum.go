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
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wum/botwatch"
	"wum/config"
	"wum/logging"
	"wum/reqcache"
	"wum/services"
	"wum/services/lguide"
	"wum/services/requests"
	"wum/services/tstorage"
	"wum/storage"

	"github.com/gorilla/mux"
)

var (
	version   string
	buildDate string
	gitCommit string
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
	if path != "" && path != "stderr" {
		logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to initialize log. File: %s", path)
		}
		log.SetOutput(logf) // runtime should close the file when program exits
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
		log.Fatal("FATAL: failed to connect to a storage database - ", err)
	}
	return db
}

func runService(conf *config.Configuration) {
	confErr := conf.Validate()
	if confErr != nil {
		log.Fatal("FATAL: ", confErr)
	}
	setupLog(conf.LogPath)
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
		log.Fatal("FATAL: ", err)
	}

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	var cache services.Cache
	if conf.Cache.RootPath != "" {
		cache = reqcache.NewReqCache(&conf.Cache)
		log.Printf("INFO: using request cache (path: %s)", conf.Cache.RootPath)

	} else {
		cache = reqcache.NewNullCache()
		log.Print("INFO: using NULL cache (path not specified)")
	}

	langGuideActions := lguide.NewLanguageGuideActions(
		&conf.LanguageGuide,
		&conf.Botwatch,
		&conf.Telemetry,
		conf.ServerReadTimeoutSecs,
		db,
		telemetryAnalyzer,
		cache,
	)
	router.HandleFunc("/language-guide", langGuideActions.Query)

	telemetryActions := tstorage.NewActions(db)
	router.HandleFunc("/telemetry", telemetryActions.Store).Methods(http.MethodPost)

	requestsActions := requests.NewActions(db)
	router.HandleFunc("/requests", requestsActions.List)

	go func() {
		evt := <-syscallChan
		exitEvent <- evt
		close(exitEvent)
	}()

	log.Printf("INFO: starting to listen at %s:%d", conf.ServerHost, conf.ServerPort)
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf("%s:%d", conf.ServerHost, conf.ServerPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Print(err)
		}
		syscallChan <- syscall.SIGTERM
	}()

	<-exitEvent
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Printf("Shutdown request error: %v", err)
	}
}

func runCleanup(conf *config.Configuration) {
	confErr := conf.Validate()
	if confErr != nil {
		log.Fatal("FATAL: ", confErr)
	}
	setupLog(conf.LogPath)
	log.Print("INFO: running cleanup procedure")
	db := initStorage(conf)
	ans := db.CleanOldData(conf.CleanupMaxAgeDays)
	if ans.Error != nil {
		log.Fatal("FATAL: failed to cleanup old records: ", ans.Error)
	}
	status, err := json.Marshal(ans)
	if err != nil {
		log.Fatal("FATAL: failed to provide cleanup summary: ", err)
	}
	log.Printf("INFO: finished old data cleanup: %s", string(status))
}

func runStatus(conf *config.Configuration, storage *storage.MySQLAdapter, ident string) {
	confErr := conf.Validate()
	if confErr != nil {
		log.Fatal("FATAL: ", confErr)
	}
	setupLog(conf.LogPath)

	ip := net.ParseIP(ident)
	var sessionID string
	if ip == nil {
		var err error
		log.Printf("assuming %s is a session ID", ident)
		sessionID = logging.NormalizeSessionID(ident)
		ip, err = storage.GetSessionIP(sessionID)
		if err != nil {
			log.Fatal("FATAL: ", err)
		}
		if ip == nil {
			log.Fatal("FATAL: no IP address found for session ", sessionID)
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
		log.Fatal("FATAL: ", err)
	}
	fakeReq, err := http.NewRequest("POST", "", nil)
	if err != nil {
		log.Fatal("FATAL: ", err)
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
			log.Fatal("FATAL: ", err)
		}
		botScore, err := telemetryAnalyzer.BotScore(fakeReq)
		if err != nil {
			log.Fatal("FATAL: ", err)
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
			log.Fatal("FATAL: ", err)
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
			"WUM - WaG UJC middleware"+
				"\n\nUsage:"+
				"\n\t%s [options] start [config.json]"+
				"\n\t%s [options] cleanup [conf.json]"+
				"\n\t%s [options] ban [ip address] [conf.json]"+
				"\n\t%s [options] unban [ip address] [conf.json]"+
				"\n\t%s [options] status [session id / IP address] [conf.json]"+
				"\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	action := flag.Arg(0)

	version := services.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}

	switch action {
	case "version":
		fmt.Printf("wag-ujc-middleware %s\nbuild date: %s\nlast commit: %s\n",
			version.Version, version.BuildDate, version.GitCommit)
		return
	case "start":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts)
		runService(conf)
	case "cleanup":
		conf := findAndLoadConfig(flag.Arg(1), cmdOpts)
		runCleanup(conf)
	case "ban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := initStorage(conf)
		if err := db.InsertBan(net.ParseIP(flag.Arg(1)), conf.BanTTLSecs); err != nil {
			log.Fatal("FATAL: ", err)
		}
	case "unban":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := initStorage(conf)
		if err := db.RemoveBan(net.ParseIP(flag.Arg(1))); err != nil {
			log.Fatal("FATAL: ", err)
		}
	case "status":
		conf := findAndLoadConfig(flag.Arg(2), cmdOpts)
		db := initStorage(conf)
		runStatus(conf, db, flag.Arg(1))

	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
