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
	"apiguard/fsops"
	"apiguard/monitoring/influx"
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
	"apiguard/services/telemetry"
	"encoding/json"
	"errors"
	"fmt"
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

type CNCAuthConf struct {
	SessionCookieName string `json:"sessionCookieName"`
}

func (services *servicesSection) validate() error {
	if services.Kontext.InternalURL != "" {
		if services.Kontext.ExternalURL == "" {
			return errors.New("missing externalUrl configuration for KonText")
		}
	}
	if services.Treq.InternalURL != "" {
		if services.Treq.ExternalURL == "" {
			return errors.New("missing externalUrl configuration for Treq")
		}
	}
	return nil
}

type Configuration struct {
	ServerHost             string                `json:"serverHost"`
	ServerPort             int                   `json:"serverPort"`
	ServerReadTimeoutSecs  int                   `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                   `json:"serverWriteTimeoutSecs"`
	TimeZone               string                `json:"timeZone"`
	Botwatch               botwatch.Conf         `json:"botwatch"`
	Telemetry              telemetry.Conf        `json:"telemetry"`
	Services               servicesSection       `json:"services"`
	Cache                  reqcache.Conf         `json:"cache"`
	Monitoring             influx.ConnectionConf `json:"monitoring"`
	LogPath                string                `json:"logPath"`
	LogLevel               string                `json:"logLevel"`
	CleanupMaxAgeDays      int                   `json:"cleanupMaxAgeDays"`
	IPBanTTLSecs           int                   `json:"IpBanTtlSecs"`
	CNCDB                  cncdb.Conf            `json:"cncDb"`
	Mail                   alarms.MailConf       `json:"mail"`
	CNCAuth                CNCAuthConf           `json:"cncAuth"`
	StatusDataDir          string                `json:"statusDataDir"`
	IgnoreStoredState      bool                  `json:"-"`
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
	if c.StatusDataDir == "" || !fsops.IsDir(c.StatusDataDir) {
		return fmt.Errorf("invalid statusDataDir: %s", c.StatusDataDir)
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
