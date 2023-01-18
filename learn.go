// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/config"
	"apiguard/ctx"

	"github.com/rs/zerolog/log"
)

func runLearn(globalCtx ctx.GlobalContext, conf *config.Configuration) {
	delayLog := cncdb.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.InfluxDB,
		delayLog,
		delayLog,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	err = telemetryAnalyzer.Learn()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
