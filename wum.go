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
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"wum/botwatch"
	"wum/config"
	"wum/fsops"
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
	Host            string
	Port            int
	ReadTimeoutSecs int
	LogPath         string
	MaxAgeDays      int
	BanSecs         int
}

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func overrideConfWithCmd(origConf *config.Configuration, cmdConf *CmdOptions) {
	if cmdConf.Host != "" {
		origConf.ServerHost = cmdConf.Host

	} else if origConf.ServerHost == "" {
		log.Printf(
			"WARNING: serverHost not specified, using default value %s",
			config.DfltServerHost,
		)
		origConf.ServerHost = config.DfltServerHost
	}
	if cmdConf.Port != 0 {
		origConf.ServerPort = cmdConf.Port

	} else if origConf.ServerPort == 0 {
		log.Printf(
			"WARNING: serverPort not specified, using default value %d",
			config.DftlServerPort,
		)
		origConf.ServerPort = config.DftlServerPort
	}
	if cmdConf.ReadTimeoutSecs != 0 {
		origConf.ServerReadTimeoutSecs = cmdConf.ReadTimeoutSecs

	} else if origConf.ServerReadTimeoutSecs == 0 {
		log.Printf(
			"WARNING: serverReadTimeoutSecs not specified, using default value %d",
			config.DfltServerReadTimeoutSecs,
		)
		origConf.ServerReadTimeoutSecs = config.DfltServerReadTimeoutSecs
	}
	if cmdConf.LogPath != "" {
		origConf.LogPath = cmdConf.LogPath

	} else if origConf.LogPath == "" {
		log.Printf("WARNING: logPath not specified, using stderr")
	}
	if cmdConf.MaxAgeDays > 0 {
		origConf.CleanupMaxAgeDays = cmdConf.MaxAgeDays

	} else if origConf.CleanupMaxAgeDays == 0 {
		log.Printf(
			"WARNING: cleanupMaxAgeDays not specified, using default value %d",
			config.DfltCleanupMaxAgeDays,
		)
		origConf.CleanupMaxAgeDays = config.DfltCleanupMaxAgeDays
	}
	if cmdConf.BanSecs > 0 {
		origConf.BanTTLSecs = cmdConf.BanSecs

	} else if origConf.BanTTLSecs == 0 {
		log.Printf(
			"WARNING: banTTLSecs not specified, using default value %d",
			config.DfltBanSecs,
		)
		origConf.BanTTLSecs = config.DfltBanSecs
	}
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
		WriteTimeout: 10 * time.Second,
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

func findAndLoadConfig(explicitPath string) *config.Configuration {
	if explicitPath != "" {
		return config.LoadConfig(explicitPath)
	}
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "conf.json")
	srchPaths := []string{
		srcPath,
		"/usr/local/etc/wum/conf.json",
		"/usr/local/etc/wum.json",
	}
	for _, path := range srchPaths {
		if fsops.IsFile(path) {
			return config.LoadConfig(path)
		}
	}
	log.Fatalf("cannot find any suitable configuration file (searched in: %s)", strings.Join(srchPaths, ", "))
	return new(config.Configuration)
}

func main() {
	rand.Seed(time.Now().Unix())
	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on (overrides conf.json)")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on (overrided conf.json)")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "read-timeout", 0, "Read timeout in seconds (overrides conf.json)")
	flag.StringVar(&cmdOpts.LogPath, "log-path", "", "A file to log to (if empty then stderr is used)")
	flag.IntVar(&cmdOpts.MaxAgeDays, "max-age-days", 0, "When cleaning old records, this specifies the oldes records (in days) to keep in database.")
	flag.IntVar(&cmdOpts.BanSecs, "ban-secs", 0, "Number of seconds to ban an IP address")
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"WUM - WaG UJC middleware\n\nUsage:\n\t%s [options] start [config.json]\n\t%s [options] cleanup [conf.json]\n\t%s [options] ban [ip address] [conf.json]\n\t%s [options] unban [ip address] [conf.json]\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
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
		conf := findAndLoadConfig(flag.Arg(1))
		overrideConfWithCmd(conf, cmdOpts)
		runService(conf)
	case "cleanup":
		conf := findAndLoadConfig(flag.Arg(1))
		overrideConfWithCmd(conf, cmdOpts)
		runCleanup(conf)
	case "ban":
		conf := findAndLoadConfig(flag.Arg(2))
		overrideConfWithCmd(conf, cmdOpts)
		db := initStorage(conf)
		if err := db.InsertBan(net.ParseIP(flag.Arg(1)), conf.BanTTLSecs); err != nil {
			log.Fatal("FATAL: ", err)
		}
	case "unban":
		conf := findAndLoadConfig(flag.Arg(2))
		overrideConfWithCmd(conf, cmdOpts)
		db := initStorage(conf)
		if err := db.RemoveBan(net.ParseIP(flag.Arg(1))); err != nil {
			log.Fatal("FATAL: ", err)
		}
	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
