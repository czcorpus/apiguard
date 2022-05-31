// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package counting

import (
	"log"
	"math"
	"net/http"
	"wum/logging"
	"wum/telemetry"
	"wum/telemetry/backend"
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
	db            backend.TelemetryStorage
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
	log.Print("WARNING: The 'counting' backend provides no learning capabilities")
	return nil
}

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	telemetry, err := a.db.LoadClientTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry, 0)
	if err != nil {
		return -1, err
	}
	if len(telemetry) == 0 {
		return -1, backend.ErrUnknownClient
	}

	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	counts := a.processTelemetry(telemetry)

	for key, rule := range a.countingRules {
		if math.Abs(float64(rule.count-counts[key])) > float64(rule.tolerance) {
			log.Printf(
				"DEBUG: invalid counts for %v. Expecting %f to be %f±%f",
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

func NewAnalyzer(db backend.TelemetryStorage) *Analyzer {
	rulesData, err := db.LoadCountingRules()
	if err != nil {
		log.Print("ERROR: ", err) // TODO return error
	}

	return &Analyzer{db: db, countingRules: prepareCountingRules(rulesData)}
}
