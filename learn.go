// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/telemetry"

	"github.com/rs/zerolog/log"
)

func runLearn(globalCtx *globctx.Context, conf *config.Configuration) {
	delayLog := guard.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	telemetryAnalyzer, err := telemetry.New(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.TimescaleDBWriter,
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
