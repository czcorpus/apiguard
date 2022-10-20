// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"path"
	"runtime"
	"strings"
	"wum/config"
	"wum/fsops"

	"github.com/rs/zerolog/log"
)

func findAndLoadConfig(explicitPath string, cmdOpts *CmdOptions, setupLog func(string)) *config.Configuration {
	var conf *config.Configuration
	if explicitPath != "" {
		conf = config.LoadConfig(explicitPath)

	} else {
		_, filepath, _, _ := runtime.Caller(0)
		srcPath := path.Join(filepath, "conf.json")
		srchPaths := []string{
			srcPath,
			"/usr/local/etc/wum/conf.json",
			"/usr/local/etc/wum.json",
		}
		for _, path := range srchPaths {
			if fsops.IsFile(path) {
				conf = config.LoadConfig(path)
				break
			}
		}
		if conf == nil {
			log.Fatal().Msgf("cannot find any suitable configuration file (searched in: %s)", strings.Join(srchPaths, ", "))
		}
	}
	setupLog(conf.LogPath)
	log.Info().Msgf("loaded configuration from %s", explicitPath)
	overrideConfWithCmd(conf, cmdOpts)
	validErr := conf.Validate()
	if validErr != nil {
		log.Fatal().Err(validErr).Msg("")
	}
	return conf
}

func overrideConfWithCmd(origConf *config.Configuration, cmdConf *CmdOptions) {
	if cmdConf.Host != "" {
		origConf.ServerHost = cmdConf.Host

	} else if origConf.ServerHost == "" {
		log.Warn().Msgf(
			"serverHost not specified, using default value %s",
			config.DfltServerHost,
		)
		origConf.ServerHost = config.DfltServerHost
	}
	if cmdConf.Port != 0 {
		origConf.ServerPort = cmdConf.Port

	} else if origConf.ServerPort == 0 {
		log.Warn().Msgf(
			"serverPort not specified, using default value %d",
			config.DftlServerPort,
		)
		origConf.ServerPort = config.DftlServerPort
	}
	if cmdConf.ReadTimeoutSecs != 0 {
		origConf.ServerReadTimeoutSecs = cmdConf.ReadTimeoutSecs

	} else if origConf.ServerReadTimeoutSecs == 0 {
		log.Warn().Msgf(
			"serverReadTimeoutSecs not specified, using default value %d",
			config.DfltServerReadTimeoutSecs,
		)
		origConf.ServerReadTimeoutSecs = config.DfltServerReadTimeoutSecs
	}
	if cmdConf.WriteTimeoutSecs != 0 {
		origConf.ServerWriteTimeoutSecs = cmdConf.WriteTimeoutSecs

	} else if origConf.ServerWriteTimeoutSecs == 0 {
		log.Warn().Msgf(
			"serverWriteTimeoutSecs not specified, using default value %d",
			config.DfltServerWriteTimeoutSecs,
		)
		origConf.ServerWriteTimeoutSecs = config.DfltServerWriteTimeoutSecs
	}
	if cmdConf.LogPath != "" {
		origConf.LogPath = cmdConf.LogPath

	} else if origConf.LogPath == "" {
		log.Warn().Msg("logPath not specified, using stderr")
	}
	if cmdConf.MaxAgeDays > 0 {
		origConf.CleanupMaxAgeDays = cmdConf.MaxAgeDays

	} else if origConf.CleanupMaxAgeDays == 0 {
		log.Warn().Msgf(
			"cleanupMaxAgeDays not specified, using default value %d",
			config.DfltCleanupMaxAgeDays,
		)
		origConf.CleanupMaxAgeDays = config.DfltCleanupMaxAgeDays
	}
	if cmdConf.BanSecs > 0 {
		origConf.BanTTLSecs = cmdConf.BanSecs

	} else if origConf.BanTTLSecs == 0 {
		log.Warn().Msgf(
			"banTTLSecs not specified, using default value %d",
			config.DfltBanSecs,
		)
		origConf.BanTTLSecs = config.DfltBanSecs
	}
}
