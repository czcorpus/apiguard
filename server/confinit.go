// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/config"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"
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

func FindAndLoadConfig(explicitPath string, cmdOpts *CmdOptions) *config.Configuration {
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
		conf.Logging.Level = logging.LogLevel(cmdOpts.LogLevel)

	} else if conf.Logging.Level == "" {
		conf.Logging.Level = "info"
	}
	logging.SetupLogging(conf.Logging)
	log.Info().Msgf("loaded configuration from %s", explicitPath)
	log.Info().Msgf("using logging level '%s'", conf.Logging.Level)
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
		origConf.Logging.Path = cmdConf.LogPath

	} else if origConf.Logging.Path == "" {
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

	if cmdConf.StreamingMode {
		log.Warn().Msg("Forcing the 'streaming' mode based on command line argument")
		origConf.OperationMode = config.OperationModeStreaming
	}

	return nil
}
