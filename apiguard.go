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
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"apiguard/cnc"
	"apiguard/common"
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/reqcache"
	"apiguard/services"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/hltscl"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	defaultConfigPath string
	version           string
	buildDate         string
	gitCommit         string
	versionInfo       = services.VersionInfo{
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
	BanDurationStr    string
	IgnoreStoredState bool
}

func (opts CmdOptions) BanDuration() (time.Duration, error) {
	// we test for '0' as the parser below does not like
	// numbers without suffix ('d', 'h', 's', ...)
	if opts.BanDurationStr == "" || opts.BanDurationStr == "0" {
		return 0, nil
	}
	return datetime.ParseDuration(opts.BanDurationStr)
}

func init() {
	if defaultConfigPath == "" {
		defaultConfigPath = "/usr/local/etc/apiguard.json"
	}
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

func openCNCDatabase(conf *cnc.Conf) *sql.DB {
	cncDB, err := cnc.OpenDB(conf)
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

func createPGPool(conf hltscl.PgConf) *pgxpool.Pool {
	conn, err := hltscl.CreatePool(conf)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	return conn
}

func createGlobalCtx(ctx context.Context, conf *config.Configuration, pgPool *pgxpool.Pool) *globctx.Context {
	ans := globctx.NewGlobalContext(ctx)

	tDBWriter := monitoring.NewTimescaleDBWriter(pgPool, conf.TimezoneLocation(), ctx)
	tDBWriter.AddTableWriter(monitoring.AlarmMonitoringTable)
	tDBWriter.AddTableWriter(monitoring.BackendMonitoringTable)
	tDBWriter.AddTableWriter(monitoring.ProxyMonitoringTable)
	tDBWriter.AddTableWriter(monitoring.TelemetryMonitoringTable)

	var cache proxy.Cache
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

	ans.TimezoneLocation = conf.TimezoneLocation()
	ans.TimescaleDBWriter = tDBWriter
	ans.BackendLogger = globctx.NewBackendLogger(tDBWriter)
	ans.CNCDB = openCNCDatabase(&conf.CNCDB)
	ans.Cache = cache
	ans.UserTableProps = conf.CNCDB.ApplyOverrides()
	return ans
}

func init() {
	gob.Register(&proxy.SimpleResponse{})
	gob.Register(&proxy.ProxiedResponse{})
}

func determineConfigPath(argPos int) string {
	v := flag.Arg(argPos)
	if v != "" {
		return v
	}
	fmt.Fprintf(os.Stderr, "using default config in %s\n", defaultConfigPath)
	return defaultConfigPath
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
	flag.StringVar(&cmdOpts.BanDurationStr, "ban-duration", "0", "A duration for the ban (e.g. 90s, 2d, 8h30m)")
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
		conf := findAndLoadConfig(determineConfigPath(1), cmdOpts)
		log.Info().
			Str("version", versionInfo.Version).
			Str("buildDate", versionInfo.BuildDate).
			Str("last commit", versionInfo.GitCommit).
			Msg("Starting CNC APIGuard")

		pgPool := createPGPool(conf.Monitoring.DB)
		runService(conf, pgPool)
	case "cleanup":
		conf := findAndLoadConfig(determineConfigPath(1), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		runCleanup(db, conf.TimezoneLocation(), conf.Limiting)
	case "ipban":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		delayLog := guard.NewDelayStats(db, conf.TimezoneLocation())
		if err := delayLog.InsertIPBan(net.ParseIP(flag.Arg(1)), conf.IPBanTTLSecs); err != nil {
			log.Fatal().Err(err).Send()
		}
	case "ipunban":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		delayLog := guard.NewDelayStats(db, conf.TimezoneLocation())
		if err := delayLog.RemoveIPBan(net.ParseIP(flag.Arg(1))); err != nil {
			log.Fatal().Err(err).Send()
		}
	case "userban":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		now := time.Now().In(conf.TimezoneLocation())
		userID, err := common.Str2UserID(flag.Arg(1))
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		banHours := 24
		_, err = guard.BanUser(
			db, conf.TimezoneLocation(), userID, nil, now, now.Add(time.Duration(banHours)*time.Hour))
		if err != nil {
			log.Error().Err(err).Msg("Failed to ban user")

		} else {
			log.Info().Msgf("Banned user %d for %d hours", userID, banHours)
		}
	case "userunban":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		db := openCNCDatabase(&conf.CNCDB)
		userID, err := strconv.Atoi(flag.Arg(1))
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		_, err = guard.UnbanUser(db, conf.TimezoneLocation(), userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to unban user")

		} else {
			log.Info().Msgf("Unbanned user %d", userID)
		}
	case "status":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		ctx := context.TODO()
		pgPool := createPGPool(conf.Monitoring.DB)
		globalCtx := createGlobalCtx(ctx, conf, pgPool)
		runStatus(globalCtx, conf, flag.Arg(1))
	case "learn":
		conf := findAndLoadConfig(determineConfigPath(1), cmdOpts)
		ctx := context.TODO()
		pgPool := createPGPool(conf.Monitoring.DB)
		globalCtx := createGlobalCtx(ctx, conf, pgPool)
		runLearn(globalCtx, conf)
	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
