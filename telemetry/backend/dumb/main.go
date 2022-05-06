// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package dumb

import (
	"log"
	"net/http"
	"wum/logging"
	"wum/telemetry/backend"
)

const (
	maxAgeSecsRelevantTelemetry = 3600 * 24 * 7
)

type Analyzer struct {
	db backend.StorageProvider
}

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) bool {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry)
	if err != nil {
		log.Print("ERROR: ", err) // TODO return error
	}
	return len(data) > 0
}

func NewAnalyzer(db backend.StorageProvider) *Analyzer {
	return &Analyzer{db: db}
}
