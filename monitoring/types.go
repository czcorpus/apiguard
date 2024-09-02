// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"apiguard/common"
	"time"

	"github.com/czcorpus/hltscl"
)

const BackendActionTypeQuery = "query"
const BackendActionTypeLogin = "login"
const BackendActionTypePreflight = "preflight"

// -----

// BackendActionType represents the most general request type distinction
// independent of a concrete service. Currently we need this mostly
// to monitor actions related to our central authentication, i.e. how
// APIGuard handles unauthenticated users and tries to authenticate them
// (if applicable)
type BackendActionType string

// -----

type ProxyProcReport struct {
	DateTime time.Time
	ProcTime float32 `json:"procTime"`
	Status   int     `json:"status"`
	Service  string  `json:"service"`
}

func (report ProxyProcReport) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(report.DateTime).
		Str("service", report.Service).
		Float("proc_time", float64(report.ProcTime)).
		Int("status", report.Status)
}

func (report ProxyProcReport) GetTime() time.Time {
	return report.DateTime
}

// -----

type TelemetryEntropy struct {
	Created                       time.Time
	SessionID                     string
	ClientIP                      string
	MAIN_TILE_DATA_LOADED         float64
	MAIN_TILE_PARTIAL_DATA_LOADED float64
	MAIN_SET_TILE_RENDER_SIZE     float64
	Score                         float64
}

func (te *TelemetryEntropy) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(te.Created).
		Str("session_id", te.SessionID).
		Str("client_ip", te.ClientIP).
		Float("MAIN_TILE_DATA_LOADED", te.MAIN_TILE_DATA_LOADED).
		Float("MAIN_TILE_PARTIAL_DATA_LOADED", te.MAIN_TILE_PARTIAL_DATA_LOADED).
		Float("MAIN_SET_TILE_RENDER_SIZE", te.MAIN_SET_TILE_RENDER_SIZE).
		Float("score", te.Score)
}

func (te *TelemetryEntropy) GetTime() time.Time {
	return te.Created
}

// ----

type BackendRequest struct {
	Created      time.Time
	Service      string
	ProcTime     float64
	IsCached     bool
	UserID       common.UserID
	IndirectCall bool
	ActionType   BackendActionType
}

func (br *BackendRequest) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(br.Created).
		Str("service", br.Service).
		Bool("is_cached", br.IsCached).
		Str("action_type", string(br.ActionType)).
		Float("proc_time", br.ProcTime).
		Bool("indirect_call", br.IndirectCall)
}

func (br *BackendRequest) GetTime() time.Time {
	return br.Created
}
