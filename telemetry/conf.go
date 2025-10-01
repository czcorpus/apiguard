// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
