// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wum/botwatch"
	"wum/config"
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
}

func runService(cmdOpts *CmdOptions) {
	conf := config.LoadConfig(flag.Arg(1))
	overrideConfWithCmd(conf, cmdOpts)

	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)

	db, err := storage.NewMySQLAdapter(
		conf.Storage.Host,
		conf.Storage.User,
		conf.Storage.Password,
		conf.Storage.Database,
	)
	if err != nil {
		log.Fatal("FATAL: failed to connect to a storage database - ", err)
	}

	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		conf.Botwatch.TelemetryAnalyzer,
		db,
	)
	if err != nil {
		log.Fatal("FATAL: ", err)
	}

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	langGuideActions := lguide.NewLanguageGuideActions(
		conf.LanguageGuide, conf.Botwatch, db, telemetryAnalyzer)
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

func main() {
	rand.Seed(time.Now().Unix())
	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on (overrides conf.json)")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on (overrided conf.json)")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "read-timeout", 0, "Read timeout in seconds (overrides conf.json)")
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"WUM - WaG UJC middleware\n\nUsage:\n\t%s [options] start config.json\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
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
		runService(cmdOpts)
	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
