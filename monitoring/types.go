// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import "time"

type TelemetryEntropy struct {
	Created                       time.Time
	SessionID                     string
	ClientIP                      string
	MAIN_TILE_DATA_LOADED         float64
	MAIN_TILE_PARTIAL_DATA_LOADED float64
	MAIN_SET_TILE_RENDER_SIZE     float64
}

func (te *TelemetryEntropy) ToInfluxDB() (map[string]string, map[string]any) {
	return map[string]string{
			"sessionID": te.SessionID,
			"clientIP":  te.ClientIP,
		},
		map[string]any{
			"MAIN_TILE_DATA_LOADED":         te.MAIN_TILE_DATA_LOADED,
			"MAIN_TILE_PARTIAL_DATA_LOADED": te.MAIN_TILE_PARTIAL_DATA_LOADED,
			"MAIN_SET_TILE_RENDER_SIZE":     te.MAIN_SET_TILE_RENDER_SIZE,
		}
}

func (te *TelemetryEntropy) GetTime() time.Time {
	return te.Created
}
