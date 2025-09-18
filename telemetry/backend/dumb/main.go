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

package dumb

import (
	"apiguard/services/logging"
	"apiguard/telemetry"
	"apiguard/telemetry/backend"
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	maxAgeSecsRelevantTelemetry = 3600 * 24 * 7
)

type Analyzer struct {
	db telemetry.Storage
}

func (a *Analyzer) Learn() error {
	log.Warn().Msg("The 'dumb' backend provides no learning capabilities")
	return nil
}

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Debug().Msgf("about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadClientTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry, 0)
	if err != nil {
		return -1, err
	}
	if len(data) == 0 {
		return 1, backend.ErrUnknownClient
	}
	return 0, nil
}

func NewAnalyzer(db telemetry.Storage) *Analyzer {
	return &Analyzer{db: db}
}
