// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neural

import (
	"fmt"
	"time"
	"wum/telemetry"
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

func normalizeTimes(actions []*telemetry.ActionRecord) *NormalizedInteraction {
	time0 := actions[0].Created
	timeDiff := actions[len(actions)-1].Created.Sub(time0)
	normalizedActions := make([]*telemetry.NormalizedActionRecord, len(actions))
	for i, item := range actions {
		nitem := telemetry.NormalizedActionRecord{
			SessionID:    item.SessionID,
			ClientIP:     item.ClientIP,
			ActionName:   item.ActionName,
			IsMobile:     item.IsMobile,
			IsSubquery:   item.IsSubquery,
			TileName:     item.TileName,
			RelativeTime: float64(item.Created.Sub(time0)) / float64(timeDiff),
		}
		normalizedActions[i] = &nitem
	}
	return &NormalizedInteraction{Actions: normalizedActions}
}
