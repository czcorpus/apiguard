// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ctx

import (
	"apiguard/monitoring"
	"apiguard/services/logging"
	"time"
)

type BackendLogger struct {
	stream           chan<- *monitoring.BackendRequest
	timezoneLocation *time.Location
}

func (b *BackendLogger) Log(service string, procTime time.Duration, cached *bool, userId *int) {
	b.stream <- &monitoring.BackendRequest{
		Created:  time.Now().In(b.timezoneLocation),
		Service:  service,
		ProcTime: procTime.Seconds(),
		IsCached: *cached,
	}
	logging.LogServiceRequest(service, procTime, cached, userId)
}

func NewBackendLogger(conf monitoring.ConnectionConf, timezoneLocation *time.Location) *BackendLogger {
	blstream := make(chan *monitoring.BackendRequest)
	go func() {
		monitoring.RunWriteConsumerSync(&conf, blstream)
	}()
	return &BackendLogger{
		stream:           blstream,
		timezoneLocation: timezoneLocation,
	}
}
