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

package main

import (
	"encoding/json"
	"time"

	"github.com/czcorpus/apiguard-common/reporting"

	"github.com/czcorpus/hltscl"
)

type PingReport struct {
	DateTime time.Time
	ProcTime float64
	Status   int
}

func (report *PingReport) ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry {
	return tableWriter.NewEntry(report.DateTime).
		Str("service", "ping").
		Float("proc_time", report.ProcTime).
		Int("status", report.Status)
}

func (report *PingReport) GetTime() time.Time {
	return report.DateTime
}

func (report *PingReport) GetTableName() string {
	return reporting.ProxyMonitoringTable
}

func (report *PingReport) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DateTime time.Time `json:"dateTime"`
		ProcTime float64   `json:"procTime"`
		Status   int       `json:"status"`
	}{
		DateTime: report.DateTime,
		ProcTime: report.ProcTime,
	})
}
