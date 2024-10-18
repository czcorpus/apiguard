// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package null

import (
	"apiguard/common"
	"apiguard/guard"
	"net/http"
)

// Null guard implements no restrictions
type Guard struct{}

func (sra *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {
	return common.InvalidUserID, nil
}

func (sra *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (guard.DelayInfo, error) {
	return guard.DelayInfo{}, nil
}

func (sra *Guard) LogAppliedDelay(respDelay guard.DelayInfo, clientID common.ClientID) error {
	return nil
}

func (sra *Guard) ClientInducedRespStatus(req *http.Request) guard.ReqProperties {
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
	}
}

// TestUserIsAnonymous must always prefer "safe evaluation" in
// case of Null Guard - i.e. it cannot say some user has non-anonymous
// rights to anything - that's why we return true here.
func (sra *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return true
}

func New() *Guard {
	return &Guard{}
}
