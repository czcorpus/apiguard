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

package main

import (
	"reflect"

	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/tlmtr"

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
	botAnalyzer, ok := telemetryAnalyzer.(guard.BotAnalyzer)
	if ok {
		if err := botAnalyzer.Learn(); err != nil {
			log.Fatal().Err(err).Msg("")
		}

	} else {
		log.Fatal().
			Msgf(
				"telemetryAnalyzer %s does not implement BotAnalyzer interface",
				reflect.TypeOf(telemetryAnalyzer),
			)
	}
}
