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

type userLimitInfo struct {
	Requests *collections.CircularList[reqCounterItem]
	Reported map[common.CheckInterval]bool
}

func (ulm *userLimitInfo) NumReqSince(interval time.Duration, loc *time.Location) int {
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
	collections.ConcurrentMap[common.UserID, *userLimitInfo]
}

func NewClientRequests() *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMap[common.UserID, *userLimitInfo](),
	}
}

func NewClientRequestsFrom(data map[common.UserID]*userLimitInfo) *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMapFrom[common.UserID, *userLimitInfo](data),
	}
}

func (cr *ClientRequests) CountRequests() (ans int) {
	cr.ForEach(func(k common.UserID, v *userLimitInfo) {
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
