// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package config

import (
	"apiguard/alarms"
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/monitoring"
	"apiguard/reqcache"
	"apiguard/services/backend/assc"
	"apiguard/services/backend/cja"
	"apiguard/services/backend/kla"
	"apiguard/services/backend/kontext"
	"apiguard/services/backend/lguide"
	"apiguard/services/backend/neomat"
	"apiguard/services/backend/psjc"
	"apiguard/services/backend/ssjc"
	"apiguard/services/backend/treq"
	"apiguard/telemetry"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	DfltServerReadTimeoutSecs  = 10
	DfltServerWriteTimeoutSecs = 30
	DftlServerPort             = 8080
	DfltServerHost             = "localhost"
	DfltCleanupMaxAgeDays      = 7
	DfltBanSecs                = 3600
	DfltTimeZone               = "Europe/Prague"
)

type servicesSection struct {
	LanguageGuide lguide.Conf  `json:"languageGuide"`
	ASSC          assc.Conf    `json:"assc"`
	SSJC          ssjc.Conf    `json:"ssjc"`
	PSJC          psjc.Conf    `json:"psjc"`
	KLA           kla.Conf     `json:"kla"`
	Neomat        neomat.Conf  `json:"neomat"`
	CJA           cja.Conf     `json:"cja"`
	Kontext       kontext.Conf `json:"kontext"`
	Treq          treq.Conf    `json:"treq"`
}

func (services *servicesSection) validate() error {
	if services.ASSC.BaseURL != "" {
		log.Info().Msgf("Service ASSC enabled")
	}
	if services.SSJC.BaseURL != "" {
		log.Info().Msgf("Service SSJC enabled")
	}
	if services.PSJC.BaseURL != "" {
		log.Info().Msgf("Service PSJC enabled")
	}
	if services.KLA.BaseURL != "" {
		log.Info().Msgf("Service KLA enabled")
	}
	if services.Neomat.BaseURL != "" {
		log.Info().Msgf("Service Neomat enabled")
	}
	if services.CJA.BaseURL != "" {
		log.Info().Msgf("Service CJA enabled")
	}
	if services.Kontext.InternalURL != "" {
		log.Info().Msgf("Service Kontext enabled")
	}
	if services.Treq.InternalURL != "" {
		log.Info().Msgf("Service Treq enabled")
	}
	return nil
}

type Configuration struct {
	ServerHost             string                    `json:"serverHost"`
	ServerPort             int                       `json:"serverPort"`
	ServerReadTimeoutSecs  int                       `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                       `json:"serverWriteTimeoutSecs"`
	TimeZone               string                    `json:"timeZone"`
	Botwatch               botwatch.Conf             `json:"botwatch"`
	Telemetry              telemetry.Conf            `json:"telemetry"`
	Services               servicesSection           `json:"services"`
	Cache                  reqcache.Conf             `json:"cache"`
	Monitoring             monitoring.ConnectionConf `json:"monitoring"`
	LogPath                string                    `json:"logPath"`
	LogLevel               string                    `json:"logLevel"`
	CleanupMaxAgeDays      int                       `json:"cleanupMaxAgeDays"`
	IPBanTTLSecs           int                       `json:"IpBanTtlSecs"`
	CNCDB                  cncdb.Conf                `json:"cncDb"`
	Mail                   alarms.MailConf           `json:"mail"`
}

func (c *Configuration) Validate() error {
	var err error
	if err = c.Botwatch.Validate("botwatch"); err != nil {
		return err
	}
	if err = c.Telemetry.Validate("telemetry"); err != nil {
		return err
	}
	if err = c.CNCDB.Validate("cncDb"); err != nil {
		return err
	}
	if err = c.Services.validate(); err != nil {
		return err
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return err
	}
	return nil
}

func (c *Configuration) TimezoneLocation() *time.Location {
	// we can ignore the error here as we always call c.Validate()
	// first (which also tries to load the location and report possible
	// error)
	loc, _ := time.LoadLocation(c.TimeZone)
	return loc
}

func LoadConfig(path string) *Configuration {
	if path == "" {
		log.Fatal().Msg("Cannot load config - path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	var conf Configuration
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	return &conf
}
