// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package telemetry

import (
	"apiguard/common"
	"apiguard/guard"
	"database/sql"
	"net"
	"time"
)

type Storage interface {
	LoadClientTelemetry(sessionID, clientIP string, maxAgeSecs, minAgeSecs int) ([]*ActionRecord, error)
	LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*IPProcData, error)
	LoadIPStats(clientIP string, maxAgeSecs int) (*IPAggData, error)
	TestIPBan(IP net.IP) (bool, error)
	LogAppliedDelay(respDelay guard.DelayInfo, clientID common.ClientID) error
	FindLearningClients(maxAgeSecs, minAgeSecs int) ([]*Client, error)
	LoadCountingRules() ([]*CountingRule, error)
	ResetStats(data *IPProcData) error
	UpdateStats(data *IPProcData) error
	CalcStatsTelemetryDiscrepancy(clientIP, sessionID string, historySecs int) (int, error)
	InsertBotLikeTelemetry(clientIP, sessionID string) error
	InsertTelemetry(transact *sql.Tx, data Payload) error
	AnalyzeDelayLog(binWidth float64, otherLimit float64) (*delayLogsHistogram, error)
	AnalyzeBans(timeAgo time.Duration) ([]banRow, error)
	StartTx() (*sql.Tx, error)
	RollbackTx(*sql.Tx) error
	CommitTx(*sql.Tx) error
}
