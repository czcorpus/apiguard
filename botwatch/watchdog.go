// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"wum/logging"
)

type StoreHandler interface {
	LoadStats(clientIP, sessionID string) (*IPProcData, error)
	UpdateStats(data *IPProcData) error
}

type Watchdog[T logging.AnyRequestRecord] struct {
	statistics     map[string]*IPProcData
	suspicions     map[string]IPProcData
	conf           BotDetectionConf
	onlineAnalysis chan T
	mutex          sync.Mutex
	db             StoreHandler
}

func (wd *Watchdog[T]) PrintStatistics() string {
	buff := strings.Builder{}
	for ip, stats := range wd.statistics {
		buff.WriteString(fmt.Sprintf("%v:\n", ip))
		buff.WriteString(fmt.Sprintf("\tcount: %d\n", stats.Count))
		buff.WriteString(fmt.Sprintf("\tmean: %01.2f\n", stats.Mean))
		buff.WriteString(fmt.Sprintf("\tstdev: %01.2f\n", stats.Stdev()))
		buff.WriteString(fmt.Sprintf("\trds: %01.2f\n", stats.Stdev()/stats.Mean))
		buff.WriteString("\n")
	}
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
	wd.mutex.Lock()
	wd.statistics = make(map[string]*IPProcData)
	wd.suspicions = make(map[string]IPProcData)
	wd.mutex.Unlock()
}

func (wd *Watchdog[T]) ResetBotCandidates() {
	wd.mutex.Lock()
	wd.suspicions = make(map[string]IPProcData)
	wd.mutex.Unlock()
}

func (wd *Watchdog[T]) Conf() BotDetectionConf {
	return wd.conf
}

func (wd *Watchdog[T]) analyze(rec T) error {
	srec, ok := wd.statistics[rec.GetClientID()]
	if !ok {
		var err error
		srec, err = wd.db.LoadStats(rec.GetClientIP().String(), rec.GetSessionID())
		if err != nil {
			return err
		}
		wd.statistics[rec.GetClientID()] = srec
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
		log.Printf("WARNING: detected suspicious statistics for %v", srec)
		prev, ok := wd.suspicions[rec.GetClientID()]
		if !ok || srec.ReqPerSecod() > prev.ReqPerSecod() {
			wd.suspicions[rec.GetClientID()] = *srec
		}
	}
	if srec.IsSuspicious(wd.conf) || rec.GetTime().Sub(srec.FirstAccess) > time.Duration(wd.conf.WatchedTimeWindowSecs)*time.Second {
		srec.FirstAccess = rec.GetTime()
		srec.Count = 0
		srec.M2 = 0
		srec.Mean = 0
	}
	srec.LastAccess = rec.GetTime()
	return wd.db.UpdateStats(srec)
}

func (wd *Watchdog[T]) GetSuspiciousRecords() []IPStats {
	wd.mutex.Lock()
	defer wd.mutex.Unlock()
	ans := make([]IPStats, 0, len(wd.suspicions))
	for ip, rec := range wd.suspicions {
		ans = append(ans, rec.ToIPStats(ip))
	}
	return ans
}

func NewLGWatchdog(conf BotDetectionConf, db StoreHandler) *Watchdog[*logging.LGRequestRecord] {
	analysis := make(chan *logging.LGRequestRecord)
	wd := &Watchdog[*logging.LGRequestRecord]{
		statistics:     make(map[string]*IPProcData),
		suspicions:     make(map[string]IPProcData),
		conf:           conf,
		onlineAnalysis: analysis,
		db:             db,
	}
	go func() {
		for item := range analysis {
			err := wd.analyze(item)
			if err != nil {
				log.Print("ERROR: ", err)
			}
		}
	}()
	return wd
}
