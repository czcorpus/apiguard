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

package lguide

import (
	"fmt"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/rs/zerolog/log"

	"github.com/czcorpus/apiguard-common/botwatch"
	"github.com/czcorpus/apiguard-common/logging"
	"github.com/czcorpus/apiguard-common/telemetry"
)

type Watchdog[T logging.AnyRequestRecord] struct {
	statistics     *collections.ConcurrentMap[string, *telemetry.IPProcData]
	suspicions     *collections.ConcurrentMap[string, telemetry.IPProcData]
	conf           *botwatch.Conf
	telemetryConf  *telemetry.Conf
	onlineAnalysis chan T
	db             telemetry.Storage
}

func (wd *Watchdog[T]) PrintStatistics() string {
	buff := strings.Builder{}
	wd.statistics.ForEach(func(ip string, stats *telemetry.IPProcData, ok bool) {
		if !ok {
			return
		}
		buff.WriteString(fmt.Sprintf("%v:\n", ip))
		buff.WriteString(fmt.Sprintf("\tcount: %d\n", stats.Count))
		buff.WriteString(fmt.Sprintf("\tmean: %01.2f\n", stats.Mean))
		buff.WriteString(fmt.Sprintf("\tstdev: %01.2f\n", stats.Stdev()))
		buff.WriteString(fmt.Sprintf("\trds: %01.2f\n", stats.Stdev()/stats.Mean))
		buff.WriteString("\n")
	})
	return buff.String()
}

func (wd *Watchdog[T]) maxLogRecordsDistance() time.Duration {
	return time.Duration(wd.conf.WatchedTimeWindowSecs/wd.conf.NumRequestsThreshold) * time.Second
}

func (wd *Watchdog[T]) Add(rec T) {
	wd.onlineAnalysis <- rec
}

func (wd *Watchdog[T]) Close() {
	close(wd.onlineAnalysis)
}

func (wd *Watchdog[T]) ResetAll() {
	wd.statistics = collections.NewConcurrentMap[string, *telemetry.IPProcData]()
	wd.suspicions = collections.NewConcurrentMap[string, telemetry.IPProcData]()
}

func (wd *Watchdog[T]) ResetBotCandidates() {
	wd.suspicions = collections.NewConcurrentMap[string, telemetry.IPProcData]()
}

func (wd *Watchdog[T]) Conf() *botwatch.Conf {
	return wd.conf
}

func (wd *Watchdog[T]) analyze(rec T) error {
	srec, ok := wd.statistics.GetWithTest(rec.GetClientID())
	if !ok {
		var err error
		srec, err = wd.db.LoadStats(
			rec.GetClientIP().String(),
			rec.GetSessionID(),
			wd.conf.WatchedTimeWindowSecs,
			false,
		)
		if err != nil {
			return err
		}
		wd.statistics.Set(rec.GetClientID(), srec)
	}
	// here we use Welford algorithm for online variance calculation
	// more info: (https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Online_algorithm)
	if srec.FirstAccess.IsZero() {
		srec.FirstAccess = rec.GetTime()
	}
	srec.Count++

	// upgrade statistics iff the current access is close enough to the last access
	if rec.GetTime().Sub(srec.LastAccess) <= wd.maxLogRecordsDistance() {
		timeDist := float64(rec.GetTime().Sub(srec.LastAccess).Milliseconds()) / 1000
		delta := timeDist - srec.Mean
		srec.Mean += delta / float64(srec.Count)
		delta2 := timeDist - srec.Mean
		srec.M2 += delta * delta2
	}
	if srec.IsSuspicious(wd.conf) {
		log.Warn().Msgf("detected suspicious statistics for %v", srec)
		prev, ok := wd.suspicions.GetWithTest(rec.GetClientID())
		if !ok || srec.ReqPerSecod() > prev.ReqPerSecod() {
			wd.suspicions.Set(rec.GetClientID(), *srec)
		}
	}
	// TODO IsSuspicious should not reset stats HERE !!!!
	if srec.IsSuspicious(wd.conf) || rec.GetTime().Sub(srec.FirstAccess) > time.Duration(wd.conf.WatchedTimeWindowSecs)*time.Second {
		srec.FirstAccess = rec.GetTime()
		srec.Count = 0
		srec.M2 = 0
		srec.Mean = 0
		srec.LastAccess = rec.GetTime()
		return wd.db.ResetStats(srec)

	} else {
		srec.LastAccess = rec.GetTime()
		return wd.db.UpdateStats(srec)
	}
}

func (wd *Watchdog[T]) GetSuspiciousRecords() []telemetry.IPStats {
	ans := make([]telemetry.IPStats, 0, wd.suspicions.Len())
	wd.suspicions.ForEach(func(ip string, rec telemetry.IPProcData, ok bool) {
		if !ok {
			return
		}
		ans = append(ans, rec.ToIPStats(ip))
	})
	return ans
}

func (wd *Watchdog[T]) assertTelemetry(rec *logging.LGRequestRecord) {
	go func() {
		if rec.SessionID != "" {
			delayDuration := time.Duration(wd.telemetryConf.DataDelaySecs) * time.Second
			time.Sleep(delayDuration)
			diff, err := wd.db.CalcStatsTelemetryDiscrepancy(rec.IPAddress, rec.SessionID, 30) // TODO
			if err != nil {
				log.Error().Err(err).Msg("failed to check for telemetry vs. stats discrepancy")
			}
			for i := 0; i < diff; i++ {
				err := wd.db.InsertBotLikeTelemetry(rec.IPAddress, rec.SessionID)
				if err != nil {
					log.Error().Err(err).Msg("failed to insert bot-like telemetry")
				}
			}
		}
	}()
}

func NewLGWatchdog(
	conf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
	db telemetry.Storage,
) *Watchdog[*logging.LGRequestRecord] {
	analysis := make(chan *logging.LGRequestRecord)
	wd := &Watchdog[*logging.LGRequestRecord]{
		statistics:     collections.NewConcurrentMap[string, *telemetry.IPProcData](),
		suspicions:     collections.NewConcurrentMap[string, telemetry.IPProcData](),
		conf:           conf,
		telemetryConf:  telemetryConf,
		onlineAnalysis: analysis,
		db:             db,
	}
	go func() {
		for item := range analysis {
			err := wd.analyze(item)
			wd.assertTelemetry(item)
			if err != nil {
				log.Error().Err(err).Msg("")
			}
		}
	}()
	return wd
}
