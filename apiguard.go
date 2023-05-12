// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"database/sql"
	"encoding/gob"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"apiguard/alarms"
	"apiguard/cncdb"
	"apiguard/common"
	"apiguard/config"
	"apiguard/ctx"
	"apiguard/services"

	"github.com/czcorpus/cnc-gokit/influx"
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
	errListen := make(chan error)
	db := influx.ConnectAPI(conf, errListen)
	go func() {
		for err := range errListen {
			log.Err(err).Msg("Failed to write measurement to InfluxDB")
		}
	}()
	log.Info().
		Str("host", conf.Server).
		Str("organization", conf.Organization).
		Str("bucket", conf.Bucket).
		Msgf("Connected to InfluxDB (v2)")
	return db
}

func preExit(alarm *alarms.AlarmTicker) {
	err := alarms.SaveState(alarm)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save alarm attributes")
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
