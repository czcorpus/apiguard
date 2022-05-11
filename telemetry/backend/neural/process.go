// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"log"
	"math"
)

type ActionBin struct {
	totalCount  int
	actionCount int
}

func calculateEntropy(interactions []*NormalizedInteraction, actionName string) (entropy float64) {
	timeBins := make(map[int]*ActionBin)
	for _, interaction := range interactions {
		for _, action := range interaction.Actions {
			timeIndex := int(action.RelativeTime * 10)
			bin, ok := timeBins[timeIndex]
			if !ok {
				bin = &ActionBin{totalCount: 0, actionCount: 0}
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
			entropy -= p * math.Log(p)
		}
	}

	log.Printf("DEBUG: `%s` entropy = %f", actionName, entropy)
	return
}
