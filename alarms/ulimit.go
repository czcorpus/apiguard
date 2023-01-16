// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/common"
	"time"
)

type userLimitInfo struct {
	Requests []reqCounterItem
	Reported map[common.CheckInterval]bool
}

func (ulm *userLimitInfo) NumReqSince(interval time.Duration, loc *time.Location) int {
	limit := time.Now().In(loc).Add(-interval)
	var ans int
	for _, v := range ulm.Requests {
		if v.created.After(limit) {
			ans++
		}
	}
	return ans
}
