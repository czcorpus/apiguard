// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reporting

import (
	"apiguard/common"
	"encoding/json"
	"time"

	"github.com/czcorpus/hltscl"
)

const ProxyMonitoringTable = "apiguard_proxy_monitoring"
const TelemetryMonitoringTable = "apiguard_telemetry_monitoring"
const BackendMonitoringTable = "apiguard_backend_monitoring"
const AlarmMonitoringTable = "apiguard_alarm_monitoring"

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
	ProcTime float64
	Status   int
	Service  string
}

func (report *ProxyProcReport) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(report.DateTime).
		Str("service", report.Service).
		Float("proc_time", report.ProcTime).
		Int("status", report.Status)
}

func (report *ProxyProcReport) GetTime() time.Time {
	return report.DateTime
}

func (report *ProxyProcReport) GetTableName() string {
	return ProxyMonitoringTable
}

func (report *ProxyProcReport) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DateTime time.Time `json:"dateTime"`
		ProcTime float64   `json:"procTime"`
		Status   int       `json:"status"`
		Service  string    `json:"service"`
	}{
		DateTime: report.DateTime,
		ProcTime: report.ProcTime,
		Status:   report.Status,
		Service:  report.Service,
	})
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

func (te *TelemetryEntropy) GetTableName() string {
	return TelemetryMonitoringTable
}

func (report *TelemetryEntropy) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Created                       time.Time `json:"created"`
		SessionID                     string    `json:"sessionId"`
		ClientIP                      string    `json:"clientIp"`
		MAIN_TILE_DATA_LOADED         float64   `json:"MAIN_TILE_DATA_LOADED"`
		MAIN_TILE_PARTIAL_DATA_LOADED float64   `json:"MAIN_TILE_PARTIAL_DATA_LOADED"`
		MAIN_SET_TILE_RENDER_SIZE     float64   `json:"MAIN_SET_TILE_RENDER_SIZE"`
		Score                         float64   `json:"Score"`
	}{
		Created:                       report.Created,
		SessionID:                     report.SessionID,
		ClientIP:                      report.ClientIP,
		MAIN_TILE_DATA_LOADED:         report.MAIN_TILE_DATA_LOADED,
		MAIN_TILE_PARTIAL_DATA_LOADED: report.MAIN_TILE_PARTIAL_DATA_LOADED,
		MAIN_SET_TILE_RENDER_SIZE:     report.MAIN_SET_TILE_RENDER_SIZE,
		Score:                         report.Score,
	})
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

func (br *BackendRequest) GetTableName() string {
	return BackendMonitoringTable
}

func (report *BackendRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Created      time.Time         `json:"created"`
		Service      string            `json:"service"`
		ProcTime     float64           `json:"procTime"`
		IsCached     bool              `json:"isCached"`
		UserID       common.UserID     `json:"userId"`
		IndirectCall bool              `json:"indirectCall"`
		ActionType   BackendActionType `json:"actionType"`
	}{
		Created:      report.Created,
		Service:      report.Service,
		ProcTime:     report.ProcTime,
		IsCached:     report.IsCached,
		UserID:       report.UserID,
		IndirectCall: report.IndirectCall,
		ActionType:   report.ActionType,
	})
}

// ----

type AlarmStatus struct {
	Created     time.Time
	Service     string
	NumUsers    int
	NumRequests int
}

func (status *AlarmStatus) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(status.Created).
		Str("service", status.Service).
		Int("num_users", status.NumUsers).
		Int("num_requests", status.NumRequests)
}

func (status *AlarmStatus) GetTime() time.Time {
	return status.Created
}

func (status *AlarmStatus) GetTableName() string {
	return AlarmMonitoringTable
}

func (report *AlarmStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Created     time.Time `json:"created"`
		Service     string    `json:"service"`
		NumUsers    int       `json:"numUsers"`
		NumRequests int       `json:"numRequests"`
	}{
		Created:     report.Created,
		Service:     report.Service,
		NumUsers:    report.NumUsers,
		NumRequests: report.NumRequests,
	})
}
