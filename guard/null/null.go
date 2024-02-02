// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package null

import (
	"apiguard/guard"
	"net/http"
)

// Null guard implements no restrictions
type Guard struct{}

func (sra *Guard) CalcDelay(req *http.Request) (guard.DelayInfo, error) {
	return guard.DelayInfo{}, nil
}

func (sra *Guard) LogAppliedDelay(respDelay guard.DelayInfo, clientIP string) error {
	return nil
}

func (sra *Guard) ClientInducedRespStatus(req *http.Request, serviceName string) guard.ReqProperties {
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
	}
}

func New() *Guard {
	return &Guard{}
}
