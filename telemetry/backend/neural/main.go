// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"fmt"
	"log"
	"net/http"
	"wum/logging"
	"wum/telemetry/backend"

	deep "github.com/patrikeh/go-deep"
)

const (
	maxAgeSecsRelevantTelemetry = 3600 * 24 * 30
)

type Analyzer struct {
	network *deep.Neural
	db      backend.StorageProvider
}

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) bool {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadTelemetry(sessionID, ip, maxAgeSecsRelevantTelemetry)
	rawIntractions := findInteractionChunks(data)
	interactions := make([]*NormalizedInteraction, len(rawIntractions))
	for i, interact := range rawIntractions {
		interactions[i] = normalizeTimes(interact.Actions)
	}
	for i, x := range interactions {
		fmt.Println("INTERACTION >>> ", i)
		for _, v := range x.Actions {
			fmt.Println("\t -> ", v)
		}
	}
	if err != nil {
		log.Print("ERROR: ", err) // TODO return error
	}
	return len(data) > 0
}

func NewAnalyzer(db backend.StorageProvider) *Analyzer {
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
	return &Analyzer{network: network, db: db}
}
