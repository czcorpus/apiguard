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

package guard

import (
	"net/http"
	"time"

	"github.com/czcorpus/apiguard-common/common"
)

type GuardType string

const (
	// UltraDuration is a reasonably high request delay which
	// can be considered an "infinite wait".
	UltraDuration = time.Duration(24) * time.Hour

	GuardTypeNull    GuardType = "null"
	GuardTypeDflt    GuardType = "dflt"
	GuardTypeCNCAuth GuardType = "cncauth"
	GuardTypeToken   GuardType = "token"
)

func (gt GuardType) IsValid() bool {
	return gt == GuardTypeNull || gt == GuardTypeDflt || gt == GuardTypeCNCAuth ||
		gt == GuardTypeToken
}

type RequestInfo struct {
	Created     time.Time     `json:"created"`
	Service     string        `json:"service"`
	NumRequests int           `json:"numRequests"`
	IP          string        `json:"ip"`
	UserID      common.UserID `json:"userId"`
}

type RequestIPCount struct {
	CountStart time.Time
	Num        int
}

func (ipc RequestIPCount) Inc() RequestIPCount {
	cs := ipc.CountStart
	if cs.IsZero() {
		cs = time.Now()
	}
	return RequestIPCount{
		CountStart: cs,
		Num:        ipc.Num + 1,
	}
}

type BotAnalyzer interface {
	Learn() error
	BotScore(req *http.Request) (float64, error)
}

