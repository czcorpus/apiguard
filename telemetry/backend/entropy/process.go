// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package entropy

import (
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

func calculateEntropy(interactions []*NormalizedInteraction, actionName string) (entropy float64) {
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
