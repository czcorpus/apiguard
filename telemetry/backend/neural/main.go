// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"apiguard/services/logging"
	"apiguard/telemetry"
	"apiguard/telemetry/backend"
	"apiguard/telemetry/backend/entropy"
	"apiguard/telemetry/preprocess"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/patrikeh/go-deep"
	"github.com/patrikeh/go-deep/training"
)

const (
	maxAgeLearn10MinBlocks = 6 * 24 * 7
)

type Analyzer struct {
	db      telemetry.Storage
	conf    *telemetry.Conf
	network *deep.Neural
}

func (a *Analyzer) importTelemetry() (training.Examples, error) {
	var ans training.Examples
	for learnBlock := 1; learnBlock <= maxAgeLearn10MinBlocks; learnBlock++ {
		maxAge := 60 * 10 * learnBlock
		minAge := 60 * 10 * (learnBlock - 1)
		clients, err := a.db.FindLearningClients(maxAge, minAge)
		if err != nil {
			return training.Examples{}, nil
		}
		for _, client := range clients {
			interactions, err := a.db.LoadClientTelemetry(client.SessionID, client.IP, maxAge, minAge)
			if len(interactions) == 0 {
				continue
			}
			log.Debug().Msgf("learnBlock %d [%d, %d], num interactions: %d", learnBlock, maxAge, minAge, len(interactions))
			if err != nil {
				return training.Examples{}, nil
			}
			normInteractions := preprocess.FindNormalizedInteractions(interactions)
			ent1 := entropy.CalculateEntropy(normInteractions, "MAIN_TILE_DATA_LOADED")
			ent2 := entropy.CalculateEntropy(normInteractions, "MAIN_TILE_PARTIAL_DATA_LOADED")
			ent3 := entropy.CalculateEntropy(normInteractions, "MAIN_SET_TILE_RENDER_SIZE")
			prevailFlag := float64(normInteractions.PrevailingLearningFlag())
			item := training.Example{
				Input:    []float64{ent1, ent2, ent3},
				Response: []float64{prevailFlag},
			}
			fmt.Println("Network input: ", item)
			ans = append(ans, item)
		}
	}
	return ans, nil
}

func (a *Analyzer) saveNetwork() error {
	dump := a.network.Dump()
	file, err := os.Create(mkDumpPath(a.conf.InternalDataPath))
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(file)
	encoder.Encode(dump)
	err = file.Close()
	return err
}

func (a *Analyzer) Learn() error {
	// params: learning rate, momentum, alpha decay, nesterov
	optimizer := training.NewSGD(0.05, 0.1, 1e-6, true)
	// params: optimizer, verbosity (print stats at every 50th iteration)
	trainer := training.NewTrainer(optimizer, 50)
	data, err := a.importTelemetry()
	if err != nil {
		return err
	}
	training, heldout := data.Split(0.6)
	trainer.Train(a.network, training, heldout, 1000) // training, validation, iterations
	return a.saveNetwork()
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
	ent1 := entropy.CalculateEntropy(interactions, "MAIN_TILE_DATA_LOADED")
	ent2 := entropy.CalculateEntropy(interactions, "MAIN_TILE_PARTIAL_DATA_LOADED")
	ent3 := entropy.CalculateEntropy(interactions, "MAIN_SET_TILE_RENDER_SIZE")
	ans := a.network.Predict([]float64{ent1, ent2, ent3})
	log.Debug().Msgf("BOT SCORE PREDICTION >>>>> %v", ans)
	return ans[0], nil
}

func mkDumpPath(internalDataPath string) string {
	return path.Join(internalDataPath, "nnetwork.bin")
}

func loadNetwork(npath string) (*deep.Neural, error) {
	file, err := os.Open(npath)
	if err != nil {
		return nil, err
	}
	decoder := gob.NewDecoder(file)
	var dump deep.Dump
	err = decoder.Decode(&dump)
	if err != nil {
		return nil, err
	}
	err = file.Close()
	if err != nil {
		return nil, err
	}
	return deep.FromDump(&dump), nil
}

func NewAnalyzer(
	db telemetry.Storage,
	conf *telemetry.Conf,
) (*Analyzer, error) {
	currDump := mkDumpPath(conf.InternalDataPath)
	var network *deep.Neural
	currDumpIsFile, err := fs.IsFile(currDump)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}
	if currDumpIsFile {
		var err error
		network, err = loadNetwork(currDump)
		if err != nil {
			return nil, err
		}

	} else {
		network = deep.NewNeural(&deep.Config{
			/* Input dimensionality */
			Inputs:     3,
			Layout:     []int{3, 3, 1},
			Activation: deep.ActivationSigmoid,
			Mode:       deep.ModeBinary,
			Weight:     deep.NewNormal(1.0, 0.0),
			Bias:       true,
		})
	}
	return &Analyzer{
		db:      db,
		conf:    conf,
		network: network,
	}, nil
}
