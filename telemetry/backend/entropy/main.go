// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package entropy

import (
	"log"
	"net/http"
	"time"
	"wum/logging"
	"wum/monitoring"
	"wum/telemetry"
	"wum/telemetry/backend"
)

type Analyzer struct {
	db         backend.StorageProvider
	conf       *telemetry.Conf
	monitoring chan<- *monitoring.TelemetryEntropy
}

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadTelemetry(sessionID, ip, a.conf.MaxAgeSecsRelevant)
	rawIntractions := findInteractionChunks(data)
	interactions := make([]*NormalizedInteraction, len(rawIntractions))
	for i, interact := range rawIntractions {
		interactions[i] = normalizeTimes(interact.Actions)
	}
	ent1 := calculateEntropy(interactions, "MAIN_TILE_DATA_LOADED")
	ent2 := calculateEntropy(interactions, "MAIN_TILE_PARTIAL_DATA_LOADED")
	ent3 := calculateEntropy(interactions, "MAIN_SET_TILE_RENDER_SIZE")
	log.Printf("DEBUG: {\"MAIN_TILE_DATA_LOADED\": %01.4f, \"MAIN_TILE_PARTIAL_DATA_LOADED\": %01.4f, \"MAIN_SET_TILE_RENDER_SIZE\": %01.4f}", ent1, ent2, ent3)
	a.monitoring <- &monitoring.TelemetryEntropy{
		Created:                       time.Now(),
		ClientIP:                      ip,
		SessionID:                     sessionID,
		MAIN_TILE_DATA_LOADED:         ent1,
		MAIN_TILE_PARTIAL_DATA_LOADED: ent2,
		MAIN_SET_TILE_RENDER_SIZE:     ent3,
	}

	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, backend.ErrUnknownClient
	}
	return 1, nil
}

func NewAnalyzer(
	db backend.StorageProvider,
	monitoringConf *monitoring.ConnectionConf,
	telemetryConf *telemetry.Conf,
) *Analyzer {
	entropyMsr := make(chan *monitoring.TelemetryEntropy)
	go func() {
		monitoring.RunWriteConsumer(monitoringConf, entropyMsr)
	}()
	return &Analyzer{
		db:         db,
		monitoring: entropyMsr,
		conf:       telemetryConf,
	}
}
