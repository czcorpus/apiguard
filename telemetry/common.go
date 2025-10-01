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
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/czcorpus/apiguard/botwatch"
	"github.com/czcorpus/apiguard/common"
)

type IPStats struct {
	IP           string  `json:"ip"`
	Mean         float64 `json:"mean"`
	Stdev        float64 `json:"stdev"`
	Count        int     `json:"count"`
	FirstRequest string  `json:"firstRequest"`
	LastRequest  string  `json:"lastRequest"`
}

func (r *IPStats) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// --------------

type IPProcData struct {
	SessionID   string    `json:"sessionID"`
	ClientIP    string    `json:"clientIP"`
	Count       int       `json:"count"`
	Mean        float64   `json:"mean"`
	M2          float64   `json:"-"`
	FirstAccess time.Time `json:"firstAccess"`
	LastAccess  time.Time `json:"lastAccess"`
}

func (ips *IPProcData) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		SessionID   string    `json:"sessionID"`
		ClientIP    string    `json:"clientIP"`
		Count       int       `json:"count"`
		Mean        float64   `json:"mean"`
		Stdev       float64   `json:"stdev"`
		FirstAccess time.Time `json:"firstAccess"`
		LastAccess  time.Time `json:"lastAccess"`
	}{
		SessionID:   ips.SessionID,
		ClientIP:    ips.ClientIP,
		Count:       ips.Count,
		Stdev:       ips.Stdev(),
		FirstAccess: ips.FirstAccess,
		LastAccess:  ips.LastAccess,
	})
}

func (ips *IPProcData) Variance() float64 {
	if ips.Count == 0 {
		return 0
	}
	return ips.M2 / float64(ips.Count)
}

func (ips *IPProcData) Stdev() float64 {
	return math.Sqrt(ips.Variance())
}

func (ips *IPProcData) ReqPerSecod() float64 {
	return float64(ips.Count) / ips.LastAccess.Sub(ips.LastAccess).Seconds()
}

func (ips *IPProcData) IsSuspicious(conf *botwatch.Conf) bool {
	return ips.Stdev()/ips.Mean <= conf.RSDThreshold && ips.Count >= conf.NumRequestsThreshold
}

func (ips *IPProcData) ToIPStats(ip string) IPStats {
	return IPStats{
		IP:           ip,
		Mean:         ips.Mean,
		Stdev:        ips.Stdev(),
		Count:        ips.Count,
		FirstRequest: ips.FirstAccess.Format(time.RFC3339),
		LastRequest:  ips.LastAccess.Format(time.RFC3339),
	}
}

// ---

type IPAggData struct {
	ClientIP    string    `json:"clientIP"`
	Count       int       `json:"count"`
	Mean        float64   `json:"mean"`
	M2          float64   `json:"-"`
	FirstAccess time.Time `json:"firstAccess"`
	LastAccess  time.Time `json:"lastAccess"`
}

func (ips *IPAggData) Variance() float64 {
	if ips.Count == 0 {
		return 0
	}
	return ips.M2 / float64(ips.Count)
}

func (ips *IPAggData) Stdev() float64 {
	return math.Sqrt(ips.Variance())
}

// -----

type CountingRule struct {
	TileName   string  `json:"tileName"`
	ActionName string  `json:"actionName"`
	Count      float32 `json:"count"`
	Tolerance  float32 `json:"tolerance"`
}

// ------

type Payload struct {
	Telemetry []*ActionRecord `json:"telemetry"`
}

// ------

type Client struct {
	SessionID string `json:"sessionId"`
	IP        string `json:"ip"`
}

// -------

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

// ------

type ActionRecord struct {
	Client       Client    `json:"client"`
	ActionName   string    `json:"actionName"`
	IsMobile     bool      `json:"isMobile"`
	IsSubquery   bool      `json:"isSubquery"`
	TileName     string    `json:"tileName"`
	Created      time.Time `json:"created"`
	TrainingFlag int       `json:"trainingFlag"`
}

// -------

type BanRow struct {
	ClientIP string `json:"clientIp"`
	Bans     int    `json:"bans"`
}

// --------

type DelayLogsHistogram struct {
	OldestRecord *time.Time     `json:"oldestRecord"`
	BinWidth     float64        `json:"binWidth"`
	OtherLimit   float64        `json:"otherLimit"`
	Data         map[string]int `json:"data"`
}

// ---------

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
	AnalyzeDelayLog(binWidth float64, otherLimit float64) (*DelayLogsHistogram, error)
	AnalyzeBans(timeAgo time.Duration) ([]BanRow, error)
	StartTx() (*sql.Tx, error)
	RollbackTx(*sql.Tx) error
	CommitTx(*sql.Tx) error
}
