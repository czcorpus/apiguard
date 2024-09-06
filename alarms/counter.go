// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/common"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
)

type UserLimitInfo struct {
	Requests    *collections.CircularList[reqCounterItem]
	NumViolated map[common.CheckInterval]int
}

func (ulm *UserLimitInfo) NumReqSince(interval time.Duration, loc *time.Location) int {
	limit := time.Now().In(loc).Add(-interval)
	var ans int
	ulm.Requests.ForEach(func(i int, item reqCounterItem) bool {
		if item.Created.After(limit) {
			ans++
		}
		return true
	})
	return ans
}

type ClientRequests struct {
	collections.ConcurrentMap[common.UserID, *UserLimitInfo]
}

func NewClientRequests() *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMap[common.UserID, *UserLimitInfo](),
	}
}

func NewClientRequestsFrom(data map[common.UserID]*UserLimitInfo) *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMapFrom(data),
	}
}

func (cr *ClientRequests) CountRequests() (ans int) {
	cr.ForEach(func(k common.UserID, v *UserLimitInfo) {
		ans += v.Requests.Len()
	})
	return
}

type serviceEntry struct {
	Conf           AlarmConf
	limits         map[common.CheckInterval]int
	Service        string
	ClientRequests *ClientRequests
}
