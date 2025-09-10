// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package globctx

import (
	"apiguard/common"
	"apiguard/reporting"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func exportURLArgs(req *http.Request) map[string]any {
	ans := make(map[string]any)
	for k, v := range req.URL.Query() {
		if len(v) == 0 || v[0] == "" {
			continue
		}
		if len(v) == 1 {
			ans[k] = v[0]

		} else {
			ans[k] = v
		}
	}
	return ans
}

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

	// Also log to the custom file logger
	event := b.fileLogger.Info().
		Bool("accessLog", true).
		Str("type", "apiguard").
		Str("service", bReq.Service).
		Float64("procTime", bReq.ProcTime).
		Bool("isCached", bReq.IsCached).
		Bool("isIndirect", bReq.IndirectCall).
		Str("actionType", string(bReq.ActionType)).
		Str("ipAddress", unireq.ClientIP(req).String()).
		Str("userAgent", req.UserAgent()).
		Any("args", exportURLArgs(req))
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
