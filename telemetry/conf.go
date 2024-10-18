// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package telemetry

import (
	"fmt"

	"github.com/czcorpus/cnc-gokit/fs"
)

type Conf struct {
	Analyzer string `json:"analyzer"`

	CustomConfPath string `json:"customConfPath"`

	// DataDelaySecs specifies a delay between WaG page load and the first
	// telemetry submit
	DataDelaySecs int `json:"dataDelaySecs"`

	// MaxAgeSecsRelevant specifies how old telemetry is considered
	// for client behavior analysis
	MaxAgeSecsRelevant int `json:"maxAgeSecsRelevant"`

	InternalDataPath string `json:"internalDataPath"`
}

func (bdc *Conf) Validate(context string) error {
	if bdc.Analyzer == "" {
		return fmt.Errorf("%s.analyzer is empty/missing", context)
	}
	if bdc.DataDelaySecs == 0 {
		return fmt.Errorf("%s.dataDelaySecs cannot be 0", context)
	}
	if bdc.MaxAgeSecsRelevant == 0 {
		return fmt.Errorf("%s.maxAgeSecsRelevant cannot be 0", context)
	}
	isDir, err := fs.IsDir(bdc.InternalDataPath)
	if err != nil {
		return fmt.Errorf("failed to test %s.internalDataPath (= %s): %w", context, bdc.InternalDataPath, err)
	}
	if !isDir {
		return fmt.Errorf("%s.internalDataPath does not specify a valid directory", context)
	}
	return nil
}
