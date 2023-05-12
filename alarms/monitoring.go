// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import "time"

type alarmStatus struct {
	created     time.Time
	service     string
	numUsers    int
	numRequests int
}

// ToInfluxDB defines a method providing data
// to be written to a database. The first returned
// value is for tags, the second one for fields.
func (status alarmStatus) ToInfluxDB() (map[string]string, map[string]any) {
	return map[string]string{
			"service": status.service,
		},
		map[string]any{
			"numUsers":    status.numUsers,
			"numRequests": status.numRequests,
		}
}

// GetTime provides a date and time when the record
// was created.
func (status alarmStatus) GetTime() time.Time {
	return status.created
}

func (aticker *AlarmTicker) GoStartMonitoring() {
	ticker := time.NewTicker(monitoringSendInterval)
	go func() {
		for range ticker.C {
			aticker.clients.ForEach(func(k string, service *serviceEntry) {
				report := alarmStatus{
					created:     time.Now().In(aticker.location),
					service:     service.Service,
					numUsers:    service.ClientRequests.Len(),
					numRequests: service.ClientRequests.CountRequests(),
				}
				aticker.monitoring.AddRecord("alarms", report)
			})
		}
	}()
}
