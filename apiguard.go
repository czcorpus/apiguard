// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"apiguard/cnc"
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard/token"
	"apiguard/proxy"
	"apiguard/proxy/cache"
	"apiguard/reporting"
	"apiguard/services"
	"apiguard/telemetry"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/hltscl"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
	StreamingMode     bool
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

func createTDBWriter(
	ctx context.Context, conf *reporting.Conf, loc *time.Location) (ans reporting.ReportingWriter) {
	if conf != nil {
		pgPool := createPGPool(conf.DB)
		ans = reporting.NewReportingWriter(pgPool, loc, ctx)

	} else {
		ans = &reporting.NullWriter{}
	}
	return
}

func createGlobalCtx(
	ctx context.Context, conf *config.Configuration, tDBWriter reporting.ReportingWriter) *globctx.Context {
	ans := globctx.NewGlobalContext(ctx)

	tDBWriter.AddTableWriter(reporting.AlarmMonitoringTable)
	tDBWriter.AddTableWriter(reporting.BackendMonitoringTable)
	tDBWriter.AddTableWriter(reporting.ProxyMonitoringTable)
	tDBWriter.AddTableWriter(reporting.TelemetryMonitoringTable)

	cncdb := openCNCDatabase(&conf.CNCDB)

	var cacheBackend proxy.Cache
	if conf.Cache.FileRootPath != "" {
		cacheBackend = cache.NewFileCache(conf.Cache)
		log.Info().Msgf("using file response cache (path: %s)", conf.Cache.FileRootPath)
		log.Warn().Msg("caching respects the Cache-Control header")

	} else if conf.Cache.RedisAddr != "" {
		cacheBackend = cache.NewRedisCache(ctx, conf.Cache)
		log.Info().Msgf("using redis response cache (addr: %s, db: %d)", conf.Cache.RedisAddr, conf.Cache.RedisDB)
		log.Warn().Msg("caching respects the Cache-Control header")

	} else {
		cacheBackend = cache.NewNullCache()
		log.Warn().Msg("using NULL cache (neither fs path nor Redis props are specified)")
	}

	ans.TimezoneLocation = conf.TimezoneLocation()
	ans.ReportingWriter = tDBWriter
	ans.BackendLogger = globctx.NewBackendLogger(tDBWriter)
	ans.CNCDB = cncdb
	ans.Cache = cacheBackend
	ans.AnonymousUserIDs = conf.CNCDB.AnonymousUserIDs

	// delay stats writer and telemetry analyzer
	ans.TelemetryDB = telemetry.NewDelayStats(ans.CNCDB, conf.TimezoneLocation())
	return ans
}

func init() {
	gob.Register(&proxy.BackendSimpleResponse{})
	gob.Register(&proxy.BackendProxiedResponse{})
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

	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on")
	flag.BoolVar(&cmdOpts.StreamingMode, "streaming-mode", false, "If used, APIGuard will run in the streaming mode no matter what is defined in its config JSON")
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
				"\n\t%s [options] status [session id / IP address] [conf.json]"+
				"\n\t%s [options] learn [conf.json]"+
				"\n\t%s generate-token"+
				"\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
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

		runService(conf)
	case "status":
		conf := findAndLoadConfig(determineConfigPath(2), cmdOpts)
		ctx := context.TODO()
		tDBWriter := createTDBWriter(ctx, conf.Reporting, conf.TimezoneLocation())
		globalCtx := createGlobalCtx(ctx, conf, tDBWriter)
		runStatus(globalCtx, conf, flag.Arg(1))
	case "learn":
		conf := findAndLoadConfig(determineConfigPath(1), cmdOpts)
		ctx := context.TODO()
		tDBWriter := createTDBWriter(ctx, conf.Reporting, conf.TimezoneLocation())
		globalCtx := createGlobalCtx(ctx, conf, tDBWriter)
		runLearn(globalCtx, conf)
	case "generate-token":
		id := uuid.New()
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			fmt.Println("failed to generate token: ", err)
			os.Exit(1)
			return
		}
		tk := base64.URLEncoding.EncodeToString(append([]byte(id.String()), bytes...))
		var tkJS token.TokenConf
		tkJS.HashedValue = fmt.Sprintf("%x", sha256.Sum256([]byte(tk)))
		tkJS.UserID = 1
		fmt.Println("token: ", tk)
		var jsonOut strings.Builder
		mrs := json.NewEncoder(&jsonOut)
		mrs.SetIndent("", "  ")
		err := mrs.Encode(tkJS)
		if err != nil {
			fmt.Println("failed to generate token: ", err)
			os.Exit(1)
			return
		}
		fmt.Printf("conf:\n%s", &jsonOut)
		return

	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
