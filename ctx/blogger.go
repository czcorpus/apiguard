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
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
)

type BackendLogger struct {
	stream           chan<- *monitoring.BackendRequest
	timezoneLocation *time.Location
}

// Log logs a service backend (e.g. KonText, Treq, some UJC server) access
// using application logging (zerolog) and also by sending data to a monitoring
// module (currently InfluxDB-based).
func (b *BackendLogger) Log(
	req *http.Request,
	service string,
	procTime time.Duration,
	cached bool,
	userID common.UserID,
	indirectCall bool,
) {
	bReq := &monitoring.BackendRequest{
		Created:      time.Now().In(b.timezoneLocation),
		Service:      service,
		ProcTime:     procTime.Seconds(),
		IsCached:     cached,
		UserID:       userID,
		IndirectCall: indirectCall,
	}
	b.stream <- bReq
	logging.LogServiceRequest(req, bReq)
}

// NewBackendLogger creates a new backend access logging service
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
