// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

import (
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
	SessionID   string
	ClientIP    string
	Count       int
	Mean        float64
	M2          float64
	FirstAccess time.Time
	LastAccess  time.Time
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

func (ips *IPProcData) IsSuspicious(conf BotDetectionConf) bool {
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
