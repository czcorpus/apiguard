// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package telemetry

type ActionRecord struct {
	SessionID   string `json:"sessionID"`
	ClientIP    string `json:"clientIP"`
	ActionName  string `json:"actionName"`
	IsMobile    bool   `json:"isMobile"`
	IsSubquery  bool   `json:"isSubquery"`
	TileName    string `json:"tileName"`
	TimestampMS int64  `json:"timestamp"`
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
