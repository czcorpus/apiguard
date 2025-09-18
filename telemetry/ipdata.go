// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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
	"apiguard/botwatch"
	"encoding/json"
	"math"
	"time"
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
