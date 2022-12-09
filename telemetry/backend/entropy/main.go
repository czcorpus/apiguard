// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package entropy

import (
	"apiguard/monitoring"
	"apiguard/services/logging"
	"apiguard/telemetry"
	"apiguard/telemetry/backend"
	"apiguard/telemetry/preprocess"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type conf struct {
	Entropies map[string]float64 `json:"entropies"`
}

func loadConf(path string) (*conf, error) {
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf conf
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

type Analyzer struct {
	db         backend.TelemetryStorage
	conf       *telemetry.Conf
	customConf *conf
	monitoring chan<- *monitoring.TelemetryEntropy
}

func (a *Analyzer) Learn() error {
	log.Warn().Msg("The 'entropy' backend provides no learning capabilities")
	return nil
}

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	log.Debug().Msgf("about to evaluate IP %s and sessionID %s", ip, sessionID)
	data, err := a.db.LoadClientTelemetry(sessionID, ip, a.conf.MaxAgeSecsRelevant, 0)
	if err != nil {
		return -1, err
	}
	if len(data) == 0 {
		return -1, backend.ErrUnknownClient
	}

	interactions := preprocess.FindNormalizedInteractions(data)
	ent1 := CalculateEntropy(interactions, "MAIN_TILE_DATA_LOADED")
	optim1 := a.customConf.Entropies["MAIN_TILE_DATA_LOADED"]
	score1 := math.Abs(ent1 - optim1)
	ent2 := CalculateEntropy(interactions, "MAIN_TILE_PARTIAL_DATA_LOADED")
	optim2 := a.customConf.Entropies["MAIN_TILE_PARTIAL_DATA_LOADED"]
	score2 := math.Abs(ent2 - optim2)
	ent3 := CalculateEntropy(interactions, "MAIN_SET_TILE_RENDER_SIZE")
	optim3 := a.customConf.Entropies["MAIN_SET_TILE_RENDER_SIZE"]
	score3 := math.Abs(ent3 - optim3)
	totalScore := math.Abs(2*1/(1+math.Exp((score1+score2+score3)/3)) - 1)
	log.Debug().Msgf("TOTAL SCORE: %01.4f avg entropy diff: %01.4f", totalScore, (score1+score2+score3)/3)
	log.Debug().Msgf("{\"MAIN_TILE_DATA_LOADED\": %01.4f, \"MAIN_TILE_PARTIAL_DATA_LOADED\": %01.4f, \"MAIN_SET_TILE_RENDER_SIZE\": %01.4f}", ent1, ent2, ent3)
	a.monitoring <- &monitoring.TelemetryEntropy{
		Created:                       time.Now(),
		ClientIP:                      ip,
		SessionID:                     sessionID,
		MAIN_TILE_DATA_LOADED:         ent1,
		MAIN_TILE_PARTIAL_DATA_LOADED: ent2,
		MAIN_SET_TILE_RENDER_SIZE:     ent3,
		Score:                         totalScore,
	}
	return totalScore, nil
}

func NewAnalyzer(
	db backend.TelemetryStorage,
	monitoringConf *monitoring.ConnectionConf,
	telemetryConf *telemetry.Conf,
) (*Analyzer, error) {
	if telemetryConf.CustomConfPath == "" {
		return nil, fmt.Errorf("missing custom configuration path for 'entropy' analyzer")
	}
	customConf, err := loadConf(telemetryConf.CustomConfPath)
	if err != nil {
		return nil, err
	}
	entropyMsr := make(chan *monitoring.TelemetryEntropy)
	go func() {
		monitoring.RunWriteConsumer(monitoringConf, entropyMsr)
	}()
	return &Analyzer{
		db:         db,
		monitoring: entropyMsr,
		conf:       telemetryConf,
		customConf: customConf,
	}, nil
}
