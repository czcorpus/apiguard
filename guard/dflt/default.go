// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package dflt

import (
	"apiguard/common"
	"apiguard/guard"
	"database/sql"
	"math"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
)

const (
	dfltCleanupInterval          = time.Duration(1) * time.Hour
	dfltNumOKRequestsPerInterval = 10
)

// Guard provides basic request protection
// based on IP counting and with some advantages
// for authenticated users.
type Guard struct {
	db                *sql.DB
	delayStats        *guard.DelayStats
	sessionCookieName string
	numRequests       *collections.ConcurrentMap[string, guard.RequestIPCount]
	clientCounter     chan common.ClientID
	cleanupInterval   time.Duration
}

func (sra *Guard) ExposeAsCounter() chan<- common.ClientID {
	return sra.clientCounter
}

func (sra *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (guard.DelayInfo, error) {
	num := sra.numRequests.Get(clientID.GetKey())
	if num.Num <= dfltNumOKRequestsPerInterval {
		return guard.DelayInfo{}, nil
	}
	d := 10 * (1/(1+math.Pow(math.E, -float64((num.Num-dfltNumOKRequestsPerInterval))/100)) - 0.5)
	return guard.DelayInfo{
			Delay: time.Duration(d*1000) * time.Millisecond,
			IsBan: false,
		},
		nil
}

func (sra *Guard) LogAppliedDelay(respDelay guard.DelayInfo, clientID common.ClientID) error {
	return nil
}

func (sra *Guard) ClientInducedRespStatus(req *http.Request, serviceName string) guard.ReqProperties {
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
	}
}

func (sra *Guard) Run() {
	for v := range sra.clientCounter {
		key := v.GetKey()
		newVal := sra.numRequests.Get(key).Inc() // we must Inc() so time is not zero
		if newVal.CountStart.Before(time.Now().Add(-sra.cleanupInterval)) {
			newVal = guard.RequestIPCount{
				CountStart: time.Now(),
				Num:        1,
			}
		}
		sra.numRequests.Set(key, newVal)
	}
}

func New(db *sql.DB, delayStats *guard.DelayStats, sessionCookieName string) *Guard {
	return &Guard{
		db:                db,
		delayStats:        delayStats,
		sessionCookieName: sessionCookieName,
		numRequests:       collections.NewConcurrentMap[string, guard.RequestIPCount](),
		clientCounter:     make(chan common.ClientID),
		cleanupInterval:   dfltCleanupInterval,
	}
}
