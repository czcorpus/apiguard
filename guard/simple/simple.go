// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package simple

import (
	"apiguard/guard"
	"net/http"
)

type SimpleRequestAnalyzer struct {
	sessionCookieName string
	numRequests       map[string]int
	counter           chan guard.RequestInfo
}

func (sra *SimpleRequestAnalyzer) ExposeAsCounter() chan guard.RequestInfo {
	return sra.counter
}

func (sra *SimpleRequestAnalyzer) CalcDelay(req *http.Request) (guard.DelayInfo, error) {
	return guard.DelayInfo{
			Delay: 0,
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

func NewSimpleRequestAnalyzer(sessionCookieName string) *SimpleRequestAnalyzer {
	return &SimpleRequestAnalyzer{
		sessionCookieName: sessionCookieName,
		numRequests:       make(map[string]int),
	}
}
