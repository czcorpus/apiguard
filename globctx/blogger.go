// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package globctx

import (
	"apiguard/common"
	"apiguard/reporting"
	"apiguard/services/logging"
	"net/http"
	"time"
)

type BackendLogger struct {
	tDBWriter reporting.ReportingWriter
}

// Log logs a service backend (e.g. KonText, Treq, some UJC server) access
// using application logging (zerolog) and also by sending data to a monitoring
// module (currently TimescaleDB).
func (b *BackendLogger) Log(
	req *http.Request,
	service string,
	procTime time.Duration,
	cached bool,
	userID common.UserID,
	indirectCall bool,
	actionType reporting.BackendActionType,
) {
	bReq := &reporting.BackendRequest{
		Created:      time.Now(),
		Service:      service,
		ProcTime:     procTime.Seconds(),
		IsCached:     cached,
		UserID:       userID,
		IndirectCall: indirectCall,
		ActionType:   actionType,
	}
	b.tDBWriter.Write(bReq)
	logging.LogServiceRequest(req, bReq)
}

// NewBackendLogger creates a new backend access logging service
func NewBackendLogger(tDBWriter reporting.ReportingWriter) *BackendLogger {
	return &BackendLogger{
		tDBWriter: tDBWriter,
	}
}
