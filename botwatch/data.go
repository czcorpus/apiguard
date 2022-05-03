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
	count       int
	mean        float64
	m2          float64
	firstAccess time.Time
	lastAccess  time.Time
}

func (ips *IPProcData) Variance() float64 {
	if ips.count == 0 {
		return 0
	}
	return ips.m2 / float64(ips.count)
}

func (ips *IPProcData) Stdev() float64 {
	return math.Sqrt(ips.Variance())
}

func (ips *IPProcData) ReqPerSecod() float64 {
	return float64(ips.count) / ips.lastAccess.Sub(ips.lastAccess).Seconds()
}

func (ips *IPProcData) IsSuspicious(conf BotDetectionConf) bool {
	return ips.Stdev()/ips.mean <= conf.RSDThreshold && ips.count >= conf.NumRequestsThreshold
}

func (ips *IPProcData) ToIPStats(ip string) IPStats {
	return IPStats{
		IP:           ip,
		Mean:         ips.mean,
		Stdev:        ips.Stdev(),
		Count:        ips.count,
		FirstRequest: ips.firstAccess.Format(time.RFC3339),
		LastRequest:  ips.lastAccess.Format(time.RFC3339),
	}
}
