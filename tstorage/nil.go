// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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

package tstorage

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard-common/telemetry"
)

type NilStorage struct{}

func (storage *NilStorage) LoadClientTelemetry(sessionID, clientIP string, maxAgeSecs, minAgeSecs int) ([]*telemetry.ActionRecord, error) {
	return []*telemetry.ActionRecord{}, nil
}

func (storage *NilStorage) LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*telemetry.IPProcData, error) {
	return nil, nil
}

func (storage *NilStorage) LoadIPStats(clientIP string, maxAgeSecs int) (*telemetry.IPAggData, error) {
	return nil, nil
}

func (storage *NilStorage) TestIPBan(IP net.IP) (bool, error) {
	return false, nil
}

func (storage *NilStorage) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	return nil
}

func (storage *NilStorage) FindLearningClients(maxAgeSecs, minAgeSecs int) ([]*telemetry.Client, error) {
	return []*telemetry.Client{}, nil
}

func (storage *NilStorage) LoadCountingRules() ([]*telemetry.CountingRule, error) {
	return []*telemetry.CountingRule{}, nil
}

func (storage *NilStorage) ResetStats(data *telemetry.IPProcData) error {
	return nil
}

func (storage *NilStorage) UpdateStats(data *telemetry.IPProcData) error {
	return nil
}

func (storage *NilStorage) CalcStatsTelemetryDiscrepancy(clientIP, sessionID string, historySecs int) (int, error) {
	return 0, nil
}

func (storage *NilStorage) InsertBotLikeTelemetry(clientIP, sessionID string) error {
	return nil
}

func (storage *NilStorage) InsertTelemetry(transact *sql.Tx, data telemetry.Payload) error {
	return nil
}

func (storage *NilStorage) AnalyzeDelayLog(binWidth float64, otherLimit float64) (*telemetry.DelayLogsHistogram, error) {
	return nil, nil
}

func (storage *NilStorage) AnalyzeBans(timeAgo time.Duration) ([]telemetry.BanRow, error) {
	return []telemetry.BanRow{}, nil
}

func (storage *NilStorage) StartTx() (*sql.Tx, error) {
	return nil, fmt.Errorf("nil storage cannot start a transaction")
}

func (storage *NilStorage) RollbackTx(*sql.Tx) error {
	return nil
}

func (storage *NilStorage) CommitTx(*sql.Tx) error {
	return nil
}

func Open(db *sql.DB, location *time.Location) telemetry.Storage {
	return &NilStorage{}
}
