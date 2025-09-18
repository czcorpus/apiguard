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

package counting

import (
	"apiguard/services/logging"
	"apiguard/telemetry"
	"apiguard/telemetry/backend"
	"math"
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	maxAgeSecsRelevantTelemetry = 3600 * 24 * 7
)

type TileActionKey struct {
	tile   string
	action string
}

type CountingRuleValue struct {
	count     float32
	tolerance float32
}

type Analyzer struct {
	db            telemetry.Storage
	countingRules map[TileActionKey]CountingRuleValue
}

func (a *Analyzer) processTelemetry(telemetry []*telemetry.ActionRecord) (counts map[TileActionKey]float32) {
	// count actions
	counts = make(map[TileActionKey]float32)
	for _, record := range telemetry {
		counts[TileActionKey{tile: record.TileName, action: record.ActionName}] += 1
	}

	// normalize actions per request
	requestCount := counts[TileActionKey{tile: "", action: "MAIN_REQUEST_QUERY_RESPONSE"}]
	for key, value := range counts {
		counts[key] = value / requestCount
	}

	return
}

func (a *Analyzer) Learn() error {
	log.Warn().Msg("The 'counting' backend provides no learning capabilities")
	return nil
}

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	tlmData, err := a.db.LoadClientTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry, 0)
	if err != nil {
		return -1, err
	}
	if len(tlmData) == 0 {
		return -1, backend.ErrUnknownClient
	}

	log.Debug().Msgf("about to evaluate IP %s and sessionID %s", ip, sessionID)
	counts := a.processTelemetry(tlmData)

	for key, rule := range a.countingRules {
		if math.Abs(float64(rule.count-counts[key])) > float64(rule.tolerance) {
			log.Debug().Msgf(
				"invalid counts for %v. Expecting %f to be %f±%f",
				key, counts[key], rule.count, rule.tolerance)
			return 1, nil
		}
	}
	return 0, nil
}

func prepareCountingRules(rulesData []*telemetry.CountingRule) (rules map[TileActionKey]CountingRuleValue) {
	rules = make(map[TileActionKey]CountingRuleValue)
	for _, rule := range rulesData {
		rules[TileActionKey{tile: rule.TileName, action: rule.ActionName}] = CountingRuleValue{count: rule.Count, tolerance: rule.Tolerance}
	}
	return
}

func NewAnalyzer(db telemetry.Storage) *Analyzer {
	rulesData, err := db.LoadCountingRules()
	if err != nil {
		log.Error().Err(err).Send() // TODO return error
	}

	return &Analyzer{db: db, countingRules: prepareCountingRules(rulesData)}
}
