// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"log"
	"net/http"
	"time"
	"wum/logging"
	"wum/monitoring"
	"wum/telemetry/backend"

	deep "github.com/patrikeh/go-deep"
)

const (
	maxAgeSecsRelevantTelemetry = 3600 * 24 * 30
)

type Analyzer struct {
	network    *deep.Neural
	db         backend.StorageProvider
	monitoring chan<- *monitoring.TelemetryEntropy
}

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry)
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

func NewAnalyzer(db backend.StorageProvider, conf *monitoring.ConnectionConf) *Analyzer {
	entropyMsr := make(chan *monitoring.TelemetryEntropy)
	go func() {
		monitoring.RunWriteConsumer(conf, entropyMsr)
	}()
	network := deep.NewNeural(&deep.Config{
		/* Input dimensionality */
		Inputs: 4,
		/* Two hidden layers consisting of two neurons each, and a single output */
		Layout: []int{3, 3, 1},
		/* Activation functions: Sigmoid, Tanh, ReLU, Linear */
		Activation: deep.ActivationSigmoid,
		/* Determines output layer activation & loss function:
		ModeRegression: linear outputs with MSE loss
		ModeMultiClass: softmax output with Cross Entropy loss
		ModeMultiLabel: sigmoid output with Cross Entropy loss
		ModeBinary: sigmoid output with binary CE loss */
		Mode: deep.ModeBinary,
		/* Weight initializers: {deep.NewNormal(μ, σ), deep.NewUniform(μ, σ)} */
		Weight: deep.NewNormal(1.0, 0.0),
		/* Apply bias */
		Bias: true,
	})
	return &Analyzer{
		network:    network,
		db:         db,
		monitoring: entropyMsr,
	}
}
