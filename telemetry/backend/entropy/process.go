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

package entropy

import (
	"apiguard/telemetry/preprocess"
	"fmt"
	"math"
)

type ActionBin struct {
	idx         int
	totalCount  int
	actionCount int
}

func (ab *ActionBin) String() string {
	return fmt.Sprintf("ActionBin{idx: %d, totalCount: %d, actionCount: %d}",
		ab.idx, ab.totalCount, ab.actionCount)
}

func CalculateEntropy(interactions []*preprocess.NormalizedInteraction, actionName string) (entropy float64) {
	timeBins := make(map[int]*ActionBin)
	for _, interaction := range interactions {
		for _, action := range interaction.Actions {
			timeIndex := int(action.RelativeTime * 10)
			bin, ok := timeBins[timeIndex]
			if !ok {
				bin = &ActionBin{idx: timeIndex, totalCount: 0, actionCount: 0}
				timeBins[timeIndex] = bin
			}
			bin.totalCount++
			if action.ActionName == actionName {
				bin.actionCount++
			}
		}
	}
	for _, bin := range timeBins {
		if bin.actionCount > 0 {
			p := float64(bin.actionCount) / float64(bin.totalCount)
			entropy -= p * math.Log2(p)
		}
	}
	return
}
