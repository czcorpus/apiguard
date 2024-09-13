// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"apiguard/reporting"
	"time"
)

func (aticker *AlarmTicker) GoStartMonitoring() {
	ticker := time.NewTicker(monitoringSendInterval)
	go func() {
		for range ticker.C {
			aticker.clients.ForEach(func(k string, service *serviceEntry, ok bool) {
				if !ok {
					return
				}
				report := &reporting.AlarmStatus{
					Created:     time.Now(),
					Service:     service.Service,
					NumUsers:    service.ClientRequests.Len(),
					NumRequests: service.ClientRequests.CountRequests(),
				}
				aticker.tDBWriter.Write(report)
			})
		}
	}()
}
