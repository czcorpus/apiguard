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

package telemetry

import (
	"fmt"
	"time"
)

type Client struct {
	SessionID string `json:"sessionId"`
	IP        string `json:"ip"`
}

type ActionRecord struct {
	Client       Client    `json:"client"`
	ActionName   string    `json:"actionName"`
	IsMobile     bool      `json:"isMobile"`
	IsSubquery   bool      `json:"isSubquery"`
	TileName     string    `json:"tileName"`
	Created      time.Time `json:"created"`
	TrainingFlag int       `json:"trainingFlag"`
}

// NormalizedActionRecord contains relativized timestamps as fractions
// from the first interaction to the last one. I.e. in case first interaction
// is at 12:00:00 and the last one at 12:30:00 and some action has a timestamp
// 12:15:00 than the normalized timestamp would be 0.5
type NormalizedActionRecord struct {
	Client       Client  `json:"client"`
	ActionName   string  `json:"actionName"`
	IsMobile     bool    `json:"isMobile"`
	IsSubquery   bool    `json:"isSubquery"`
	TileName     string  `json:"tileName"`
	RelativeTime float64 `json:"relativeTime"`
	TrainingFlag int     `json:"trainingFlag"`
}

func (nar *NormalizedActionRecord) String() string {
	return fmt.Sprintf(
		"NormalizedActionRecord{SessionID: %s, ClientIP: %s, ActionName: %s, RelativeTime: %01.2f",
		nar.Client.SessionID, nar.Client.IP, nar.ActionName, nar.RelativeTime)
}

type Payload struct {
	Telemetry []*ActionRecord `json:"telemetry"`
}

type CountingRule struct {
	TileName   string  `json:"tileName"`
	ActionName string  `json:"actionName"`
	Count      float32 `json:"count"`
	Tolerance  float32 `json:"tolerance"`
}
