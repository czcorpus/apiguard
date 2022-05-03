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
)

const (
	DfltServerReadTimeoutSecs = 30
	DftlServerPort            = 8080
	DfltServerHost            = "localhost"
)

type LanguageGuideConf struct {
	BaseURL         string `json:"baseURL"`
	ClientUserAgent string `json:"clientUserAgent"`
}

type Configuration struct {
	ServerHost            string                    `json:"serverHost"`
	ServerPort            int                       `json:"serverPort"`
	ServerReadTimeoutSecs int                       `json:"serverReadTimeoutSecs"`
	Botwatch              botwatch.BotDetectionConf `json:"botwatch"`
	LanguageGuide         LanguageGuideConf         `json:"languageGuide"`
}

func LoadConfig(path string) *Configuration {
	if path == "" {
		log.Fatal("FATAL: Cannot load config - path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	var conf Configuration
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	return &conf
}
