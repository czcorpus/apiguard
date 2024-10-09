// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/config"
	"apiguard/globctx"
	"apiguard/guard/tlmtr"

	"github.com/rs/zerolog/log"
)

func runLearn(globalCtx *globctx.Context, conf *config.Configuration) {
	telemetryAnalyzer, err := tlmtr.New(
		globalCtx,
		&conf.Botwatch,
		conf.Telemetry,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	err = telemetryAnalyzer.Learn()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
