// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ctx

import (
	"apiguard/common"
	"apiguard/monitoring"
	"apiguard/services/logging"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
)

type BackendLogger struct {
	stream           chan<- *monitoring.BackendRequest
	timezoneLocation *time.Location
}

func (b *BackendLogger) Log(
	service string,
	procTime time.Duration,
	cached bool,
	userID *common.UserID,
	indirectCall bool,
) {
	b.stream <- &monitoring.BackendRequest{
		Created:      time.Now().In(b.timezoneLocation),
		Service:      service,
		ProcTime:     procTime.Seconds(),
		IsCached:     cached,
		UserID:       int(*userID),
		IndirectCall: indirectCall,
	}
	logging.LogServiceRequest(service, procTime, cached, userID)
}

func NewBackendLogger(db *influx.InfluxDBAdapter, timezoneLocation *time.Location) *BackendLogger {
	blstream := make(chan *monitoring.BackendRequest)
	go func() {
		influx.RunWriteConsumerSync(db, "state", blstream)
	}()
	return &BackendLogger{
		stream:           blstream,
		timezoneLocation: timezoneLocation,
	}
}
