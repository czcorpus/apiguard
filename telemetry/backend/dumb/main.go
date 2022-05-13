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

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry)
	if err != nil {
		return -1, err
	}
	if len(data) == 0 {
		return 1, backend.ErrUnknownClient
	}
	return 0, nil
}

func NewAnalyzer(db backend.StorageProvider) *Analyzer {
	return &Analyzer{db: db}
}
