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
	log.Printf("DEBUG: `%s` entropy = %f", actionName, entropy)
	log.Printf("\tCorresponding uniform entropy = %f", -math.Log(1/float64(totalCount)))
	log.Printf("\tCorresponding uniform entropy per interaction = %f", -math.Log(1/(float64(totalCount)/float64(len(interactions)))))
	return
}
