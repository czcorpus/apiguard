// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package telemetry

import "time"

type ActionRecord struct {
	SessionID  string    `json:"sessionID"`
	ClientIP   string    `json:"clientIP"`
	ActionName string    `json:"actionName"`
	IsMobile   bool      `json:"isMobile"`
	IsSubquery bool      `json:"isSubquery"`
	TileName   string    `json:"tileName"`
	Created    time.Time `json:"created"`
}

// NormalizedActionRecord contains relativized timestamps as fractions
// from the first interaction to the last one. I.e. in case first interaction
// is at 12:00:00 and the last one at 12:30:00 and some action has a timestamp
// 12:15:00 than the normalized timestamp would be 0.5
type NormalizedActionRecord struct {
	SessionID    string  `json:"sessionID"`
	ClientIP     string  `json:"clientIP"`
	ActionName   string  `json:"actionName"`
	IsMobile     bool    `json:"isMobile"`
	IsSubquery   bool    `json:"isSubquery"`
	TileName     string  `json:"tileName"`
	RelativeTime float64 `json:"relativeTime"`
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
