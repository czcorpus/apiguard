// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package simple

import (
	"apiguard/guard"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
)

const (
	dfltCleanupInterval          = time.Duration(1) * time.Hour
	dfltNumOKRequestsPerInterval = 10
)

type SimpleRequestAnalyzer struct {
	sessionCookieName string
	numRequests       *collections.ConcurrentMap[string, guard.RequestIPCount]
	ipCounter         chan string
	cleanupInterval   time.Duration
}

func (sra *SimpleRequestAnalyzer) ExposeAsCounter() chan<- string {
	return sra.ipCounter
}

func (sra *SimpleRequestAnalyzer) CalcDelay(req *http.Request) (guard.DelayInfo, error) {
	ip := strings.SplitN(req.RemoteAddr, ":", 2)[0]
	num := sra.numRequests.Get(ip)
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

func (sra *SimpleRequestAnalyzer) LogAppliedDelay(respDelay guard.DelayInfo, clientIP string) error {
	return nil
}

func (sra *SimpleRequestAnalyzer) UserInducedResponseStatus(req *http.Request, serviceName string) guard.ReqProperties {
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
	}
}

func (sra *SimpleRequestAnalyzer) Run() {
	for v := range sra.ipCounter {
		newVal := sra.numRequests.Get(v).Inc() // we must Inc() so time is not zero
		if newVal.CountStart.Before(time.Now().Add(-sra.cleanupInterval)) {
			newVal = guard.RequestIPCount{
				CountStart: time.Now(),
				Num:        1,
			}
		}
		sra.numRequests.Set(v, newVal)
	}
}

func NewSimpleRequestAnalyzer(sessionCookieName string) *SimpleRequestAnalyzer {
	return &SimpleRequestAnalyzer{
		sessionCookieName: sessionCookieName,
		numRequests:       collections.NewConcurrentMap[string, guard.RequestIPCount](),
		ipCounter:         make(chan string),
		cleanupInterval:   dfltCleanupInterval,
	}
}
