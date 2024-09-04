// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/monitoring"
	"time"

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
	return monitoring.ProxyMonitoringTable
}
