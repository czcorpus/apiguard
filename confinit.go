// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/config"
	"path"
	"runtime"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

func findAndLoadConfig(explicitPath string, cmdOpts *CmdOptions) *config.Configuration {
	var conf *config.Configuration
	if explicitPath != "" {
		conf = config.LoadConfig(explicitPath)

	} else {
		_, filepath, _, _ := runtime.Caller(0)
		srcPath := path.Join(filepath, "conf.json")
		srchPaths := []string{
			srcPath,
			"/usr/local/etc/apiguard/conf.json",
			"/usr/local/etc/apiguard.json",
		}
		for _, path := range srchPaths {
			isFile, err := fs.IsFile(path)
			if err != nil {
				log.Fatal().Msgf(
					"error when searching for asuitable configuration file (searched in: %s): %s",
					strings.Join(srchPaths, ", "),
					err,
				)
			}
			if isFile {
				conf = config.LoadConfig(path)
				break
			}
		}
		if conf == nil {
			log.Fatal().Msgf("cannot find any suitable configuration file (searched in: %s)", strings.Join(srchPaths, ", "))
		}
	}
	if cmdOpts.LogLevel != "" {
		conf.LogLevel = cmdOpts.LogLevel

	} else if conf.LogLevel == "" {
		conf.LogLevel = "info"
	}
	setupLog(conf.LogPath, conf.LogLevel)
	log.Info().Msgf("loaded configuration from %s", explicitPath)
	log.Info().Msgf("using logging level '%s'", conf.LogLevel)
	applyDefaults(conf)
	err := overrideConfWithCmd(conf, cmdOpts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize configuration")
	}
	validErr := conf.Validate()
	if validErr != nil {
		log.Fatal().Err(validErr).Msg("")
	}
	return conf
}

// applyDefaults applies default values for optional config items
// not handled by overrideConfWithCmd (i.e. items not configurable
// via command line arguments).
func applyDefaults(conf *config.Configuration) {
	if conf.TimeZone == "" {
		conf.TimeZone = config.DfltTimeZone
		log.Warn().Msgf("timeZone not specified, using default: %s", conf.TimeZone)
	}
}

func overrideConfWithCmd(origConf *config.Configuration, cmdConf *CmdOptions) error {
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
	banDuration, err := cmdConf.BanDuration()
	if err != nil {
		return err
	}
	if banDuration > 0 {
		origConf.IPBanTTLSecs = int(banDuration.Seconds())

	} else if origConf.IPBanTTLSecs == 0 {
		log.Warn().Msgf(
			"IPBanTTLSecs not specified, using default value %d",
			config.DfltBanSecs,
		)
		origConf.IPBanTTLSecs = config.DfltBanSecs
	}

	if cmdConf.IgnoreStoredState {
		log.Warn().Msg("Based on a request, stored alarm/counter state will not be loaded")
		origConf.IgnoreStoredState = cmdConf.IgnoreStoredState
	}
	return nil
}
