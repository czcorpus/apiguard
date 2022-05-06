// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

import (
	"fmt"
	"net/http"
	"wum/telemetry/backend"
	"wum/telemetry/backend/counting"
	"wum/telemetry/backend/dumb"
	"wum/telemetry/backend/neural"
)

type Backend interface {
	Learn(req *http.Request, isLegit bool)
	Evaluate(req *http.Request) bool
}

type Analyzer struct {
	backend Backend
}

func (a *Analyzer) Analyze(req *http.Request) (bool, error) {
	return a.backend.Evaluate(req), nil // TODO
}

func NewAnalyzer(backendType string, db backend.StorageProvider) (*Analyzer, error) {
	switch backendType {
	case "counting":
		return &Analyzer{backend: counting.NewAnalyzer(db)}, nil
	case "dumb":
		return &Analyzer{backend: dumb.NewAnalyzer(db)}, nil
	case "neural":
		return &Analyzer{backend: neural.NewAnalyzer(db)}, nil
	default:
		return nil, fmt.Errorf("unknown analyzer backend %s", backendType)
	}
}
