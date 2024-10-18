// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

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
