// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Department of Linguistics,
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
	"apiguard/common"
	"database/sql"
	"net"
	"time"
)

type Storage interface {
	LoadClientTelemetry(sessionID, clientIP string, maxAgeSecs, minAgeSecs int) ([]*ActionRecord, error)
	LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*IPProcData, error)
	LoadIPStats(clientIP string, maxAgeSecs int) (*IPAggData, error)
	TestIPBan(IP net.IP) (bool, error)
	LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error
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
