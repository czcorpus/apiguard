// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"wum/logging"
	"wum/telemetry/backend"
	"wum/telemetry/backend/counting"
	"wum/telemetry/backend/dumb"
	"wum/telemetry/backend/neural"
)

type Backend interface {
	Learn(req *http.Request, isLegit bool)

	// Evaluate should evaluate client legitimacy using
	// values -1 and an interval 0 to 1 where:
	// * 0 = non-legit client and
	// * 1 = perfectly legit client
	// In case the returned error is ErrUnknownClient
	Evaluate(req *http.Request) (float64, error)
}

type StatsStorage interface {
	LoadStats(clientIP, sessionID string) (*IPProcData, error)
	LoadIPStats(clientIP string) (*IPProcData, error)
}

type Analyzer struct {
	backend Backend
	storage StatsStorage
}

func (a *Analyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	humanLevel, err := a.backend.Evaluate(req)
	if err == backend.ErrUnknownClient {
		log.Print("DEBUG: client without telemetry")
		// no telemetry - let's check client's request activity
		ip, sessionID := logging.ExtractRequestIdentifiers(req)
		stats, err := a.storage.LoadStats(ip, sessionID)
		if err != nil {
			return 0, err
		}
		if stats.Count > 50 {
			// user with a "long" session waits from 10s to infinity
			return time.Duration(stats.Count/5) * time.Second, nil

		} else if stats.Count > 5 {
			// user with a "long" session and just a few requests
			// waits for ~ 2.5 -- 10 seconds
			return time.Duration(stats.Count/2) * time.Second, nil

		} else {
			stats, err := a.storage.LoadIPStats(ip)
			if err != nil {
				return 0, err
			}
			// user w
			return time.Duration(stats.Count/4) * time.Second, nil
		}

	} else if err != nil {
		return 0, err

	} else {
		log.Print("DEBUG: Client with telemetry...")
		// user with telemetry waits from 0 to 25 s
		return time.Duration((5 - 5*humanLevel) * (5 - 5*humanLevel)), nil
	}
}

func NewAnalyzer(
	backendType string,
	db backend.StorageProvider,
	statsStorage StatsStorage,
) (*Analyzer, error) {
	switch backendType {
	case "counting":
		return &Analyzer{
			backend: counting.NewAnalyzer(db),
			storage: statsStorage,
		}, nil
	case "dumb":
		return &Analyzer{
			backend: dumb.NewAnalyzer(db),
			storage: statsStorage,
		}, nil
	case "neural":
		return &Analyzer{
			backend: neural.NewAnalyzer(db),
			storage: statsStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unknown analyzer backend %s", backendType)
	}
}
