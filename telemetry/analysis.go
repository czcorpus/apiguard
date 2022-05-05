// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package telemetry

import (
	"fmt"
	"net/http"
	"wum/telemetry/backend/counting"
)

type Backend interface {
	Learn(req *http.Request, isLegit bool)
	Evaluate(req *http.Request) bool
}

type Analyzer struct {
	backend Backend
}

func NewAnalyzer(backendType string) (*Analyzer, error) {
	switch backendType {
	case "counting":
		return &Analyzer{backend: &counting.Analyzer{}}, nil
	default:
		return nil, fmt.Errorf("unknown analyzer backend %s", backendType)
	}
}
