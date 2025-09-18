// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package null

import (
	"apiguard/common"
	"apiguard/guard"
	"net/http"
	"time"
)

// Null guard implements no restrictions
type Guard struct{}

func (sra *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {
	return common.InvalidUserID, nil
}

func (sra *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error) {
	return 0, nil
}

func (sra *Guard) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	return nil
}

func (sra *Guard) EvaluateRequest(req *http.Request, fallbackCookie *http.Cookie) guard.ReqEvaluation {
	return guard.ReqEvaluation{
		ProposedResponse: http.StatusOK,
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
