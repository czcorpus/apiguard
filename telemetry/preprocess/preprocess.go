// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package preprocess

import (
	"apiguard/telemetry"
	"fmt"
	"time"
)

type Interaction struct {
	Actions []*telemetry.ActionRecord
}

func (i *Interaction) String() string {
	return fmt.Sprintf(
		"Interaction from %v to %v, num. actions: %d",
		i.Actions[0].Created,
		i.Actions[len(i.Actions)-1].Created,
		len(i.Actions),
	)
}

type NormalizedInteraction struct {
	Actions []*telemetry.NormalizedActionRecord
}

type NormalizedInteractionList []*NormalizedInteraction

func (ni NormalizedInteractionList) PrevailingLearningFlag() int {
	numTrue := 0
	numFalse := 0
	for _, interact := range ni {
		for _, item := range interact.Actions {
			if item.TrainingFlag > 0 {
				numTrue++

			} else if item.TrainingFlag == 0 {
				numFalse++
			}
		}
	}
	if numTrue > 2*numFalse {
		return 1

	} else if numFalse > 2*numTrue {
		return 0
	}
	return -1
}

func findPreReqActions(startIdx int, data []*telemetry.ActionRecord) []*telemetry.ActionRecord {
	startTime := data[startIdx].Created
	ans := make([]*telemetry.ActionRecord, 0, 100)
	for i := startIdx - 1; i > 0; i-- {
		action := data[i]
		if action.ActionName == "MAIN_SET_TILE_RENDER_SIZE" {
			if startTime.Sub(action.Created) < time.Duration(500)*time.Millisecond {
				ans = append(ans, action)
			}

		} else {
			break
		}
	}
	return ans
}

func FindInteractionChunks(data []*telemetry.ActionRecord) []*Interaction {
	ans := make([]*Interaction, 0, 50)
	var currInteraction *Interaction
	for i, item := range data {
		if item.ActionName == "MAIN_SET_TILE_RENDER_SIZE" {
			continue

		} else if item.ActionName == "MAIN_REQUEST_QUERY_RESPONSE" {
			if currInteraction != nil {
				ans = append(ans, currInteraction)
			}
			currInteraction = &Interaction{
				Actions: findPreReqActions(i, data),
			}

		} else if currInteraction != nil {
			currInteraction.Actions = append(currInteraction.Actions, item)
		}
	}
	if currInteraction != nil {
		ans = append(ans, currInteraction)
	}
	return ans
}

func normalizeTimes(actions []*telemetry.ActionRecord) *NormalizedInteraction {
	if len(actions) == 0 {
		return &NormalizedInteraction{Actions: []*telemetry.NormalizedActionRecord{}}
	}
	time0 := actions[0].Created
	timeDiff := actions[len(actions)-1].Created.Sub(time0)
	normalizedActions := make([]*telemetry.NormalizedActionRecord, len(actions))
	for i, item := range actions {
		nitem := telemetry.NormalizedActionRecord{
			Client:       item.Client,
			ActionName:   item.ActionName,
			IsMobile:     item.IsMobile,
			IsSubquery:   item.IsSubquery,
			TileName:     item.TileName,
			TrainingFlag: item.TrainingFlag,
			RelativeTime: float64(item.Created.Sub(time0)) / float64(timeDiff),
		}
		normalizedActions[i] = &nitem
	}
	return &NormalizedInteraction{Actions: normalizedActions}
}

func findInteractionChunks(data []*telemetry.ActionRecord) []*Interaction {
	ans := make([]*Interaction, 0, 50)
	var currInteraction *Interaction
	for i, item := range data {
		if item.ActionName == "MAIN_SET_TILE_RENDER_SIZE" {
			continue

		} else if item.ActionName == "MAIN_REQUEST_QUERY_RESPONSE" {
			if currInteraction != nil {
				ans = append(ans, currInteraction)
			}
			currInteraction = &Interaction{
				Actions: findPreReqActions(i, data),
			}

		} else if currInteraction != nil {
			currInteraction.Actions = append(currInteraction.Actions, item)
		}
	}
	if currInteraction != nil {
		ans = append(ans, currInteraction)
	}
	return ans
}

func FindNormalizedInteractions(data []*telemetry.ActionRecord) NormalizedInteractionList {
	rawIntractions := findInteractionChunks(data)
	actions := make([]*NormalizedInteraction, len(rawIntractions))
	for i, interact := range rawIntractions {
		actions[i] = normalizeTimes(interact.Actions)
	}
	return actions
}
