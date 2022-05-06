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
	db            backend.StorageProvider
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

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) bool {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	telemetry, err := a.db.LoadTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry)
	if err != nil {
		log.Print("ERROR: ", err) // TODO return error
	}

	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	counts := a.processTelemetry(telemetry)

	for key, rule := range a.countingRules {
		if math.Abs(float64(rule.count-counts[key])) > float64(rule.tolerance) {
			log.Printf(
				"DEBUG: invalid counts for %v. Expecting %f to be %f±%f",
				key, counts[key], rule.count, rule.tolerance)
			return false
		}
	}
	return true
}

func prepareCountingRules(rulesData []*telemetry.CountingRule) (rules map[TileActionKey]CountingRuleValue) {
	rules = make(map[TileActionKey]CountingRuleValue)
	for _, rule := range rulesData {
		rules[TileActionKey{tile: rule.TileName, action: rule.ActionName}] = CountingRuleValue{count: rule.Count, tolerance: rule.Tolerance}
	}
	return
}

func NewAnalyzer(db backend.StorageProvider) *Analyzer {
	rulesData, err := db.LoadCountingRules()
	if err != nil {
		log.Print("ERROR: ", err) // TODO return error
	}

	return &Analyzer{db: db, countingRules: prepareCountingRules(rulesData)}
}
