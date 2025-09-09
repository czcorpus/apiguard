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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type BackendLogger struct {
	tDBWriter  reporting.ReportingWriter
	fileLogger zerolog.Logger
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
	if b == nil {
		log.Error().Msg("trying to call nil backend logger - ignoring")
		return
	}
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

	// Also log to the custom file logger
	event := b.fileLogger.Info().
		Bool("accessLog", true).
		Str("type", "apiguard").
		Str("service", bReq.Service).
		Float64("procTime", bReq.ProcTime).
		Bool("isCached", bReq.IsCached).
		Bool("isIndirect", bReq.IndirectCall).
		Str("actionType", string(bReq.ActionType))
	if bReq.UserID.IsValid() {
		event.Int("userId", int(bReq.UserID))
	}
	event.Send()
}

// NewBackendLogger creates a new backend access logging service
func NewBackendLogger(tDBWriter reporting.ReportingWriter, logPath string) (*BackendLogger, error) {
	// Use default logger if logPath is empty
	if logPath == "" {
		return &BackendLogger{
			tDBWriter:  tDBWriter,
			fileLogger: log.Logger,
		}, nil
	}

	// Create or open the log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend logger with file %s: %w", logPath, err)
	}

	// Create a new zerolog logger that writes to the file
	fileLogger := zerolog.New(file).With().Timestamp().Logger()

	return &BackendLogger{
		tDBWriter:  tDBWriter,
		fileLogger: fileLogger,
	}, nil
}
