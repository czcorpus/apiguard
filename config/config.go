// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"wum/botwatch"
	"wum/monitoring"
	"wum/reqcache"
	"wum/services/assc"
	"wum/services/lguide"
	"wum/storage"
	"wum/telemetry"
)

const (
	DfltServerReadTimeoutSecs  = 10
	DfltServerWriteTimeoutSecs = 30
	DftlServerPort             = 8080
	DfltServerHost             = "localhost"
	DfltCleanupMaxAgeDays      = 7
	DfltBanSecs                = 3600
)

type servicesSection struct {
	LanguageGuide lguide.Conf `json:"languageGuide"`
	ASSC          assc.Conf   `json:"assc"`
}

type Configuration struct {
	ServerHost             string                    `json:"serverHost"`
	ServerPort             int                       `json:"serverPort"`
	ServerReadTimeoutSecs  int                       `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                       `json:"serverWriteTimeoutSecs"`
	Botwatch               botwatch.Conf             `json:"botwatch"`
	Telemetry              telemetry.Conf            `json:"telemetry"`
	Storage                storage.Conf              `json:"storage"`
	Services               servicesSection           `json:"services"`
	Cache                  reqcache.Conf             `json:"cache"`
	Monitoring             monitoring.ConnectionConf `json:"monitoring"`
	LogPath                string                    `json:"logPath"`
	CleanupMaxAgeDays      int                       `json:"cleanupMaxAgeDays"`
	BanTTLSecs             int                       `json:"banTTLSecs"`
}

func (c *Configuration) Validate() error {
	var err error
	err = c.Botwatch.Validate("botwatch")
	if err != nil {
		return err
	}
	err = c.Telemetry.Validate("telemetry")
	if err != nil {
		return err
	}
	err = c.Storage.Validate("storage")
	if err != nil {
		return err
	}
	err = c.Services.LanguageGuide.Validate("services/languageGuide")
	if err != nil {
		return err
	}
	err = c.Services.LanguageGuide.Validate("services/assc")
	if err != nil {
		return err
	}
	return nil
}

func LoadConfig(path string) *Configuration {
	if path == "" {
		log.Fatal("FATAL: Cannot load config - path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	log.Printf("INFO: loaded configuration from %s", path)
	var conf Configuration
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	return &conf
}
