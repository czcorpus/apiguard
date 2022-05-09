// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"math"
)

func calculateEntropy(interactions []*NormalizedInteraction, actionName string) (entropy float64) {
	totalCount := 0
	timeCount := make(map[int]int)
	for _, interaction := range interactions {
		for _, action := range interaction.Actions {
			if action.ActionName == actionName {
				timeCount[int(action.RelativeTime*10)]++
				totalCount++
			}
		}
	}

	for _, count := range timeCount {
		p := float64(count) / float64(totalCount)
		entropy -= p * math.Log(p)
	}
	return
}
