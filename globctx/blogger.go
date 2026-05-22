// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package globctx

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/klogproc-core/save/elastic"
	"github.com/czcorpus/klogproc-core/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	defaultBufferSize = 1000
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

// -----------------------

type ElasticArchiveConf struct {
	*elastic.ConnectionConf

	// BufferSize defines a respective channel buffer size
	// used in the queue between proxy producing log records
	// and the Elastic writer
	BufferSize int `json:"bufferSize"`
}

func (eac *ElasticArchiveConf) AsConnectionConf() *elastic.ConnectionConf {
	return eac.ConnectionConf
}

func (eac *ElasticArchiveConf) Validate() error {
	if err := eac.ConnectionConf.Validate(); err != nil {
		return err
	}
	if eac.BufferSize == 0 {
		eac.BufferSize = defaultBufferSize
		log.Warn().Int("value", defaultBufferSize).Msg("bufferSize not set, using default")

	} else if eac.BufferSize < 0 {
		return fmt.Errorf("invalid bufferSize: %d", eac.BufferSize)
	}
	return nil
}

// ------

type BackendLogger struct {
	tDBWriter     reporting.ReportingWriter
	fileLogger    zerolog.Logger
	reqPathPrefix string
	esDispatcher  *ESDispatcher
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
	firstPartyCall bool,
	actionType reporting.BackendActionType,
) {
	if b == nil {
		log.Error().Msg("trying to call nil backend logger - ignoring")
		return
	}
	bReq := &reporting.BackendRequest{
		Created:        time.Now(),
		Service:        service,
		ProcTime:       procTime.Seconds(),
		IsCached:       cached,
		UserID:         userID,
		FirstPartyCall: firstPartyCall,
		ActionType:     actionType,
	}
	b.tDBWriter.Write(bReq)
	// Also log to the custom file logger
	event := b.fileLogger.Info().
		Bool("accessLog", true).
		Str("type", "apiguard").
		Str("service", bReq.Service).
		Float64("procTime", bReq.ProcTime).
		Bool("isCached", bReq.IsCached).
		Bool("isFirstPartyCall", bReq.FirstPartyCall).
		Str("actionType", string(bReq.ActionType)).
		Str("ipAddress", unireq.ClientIP(req).String()).
		Str("userAgent", req.UserAgent()).
		Str("requestPath", strings.TrimPrefix(req.URL.Path, b.reqPathPrefix)).
		Any("args", exportURLArgs(req))
	if bReq.UserID.IsValid() {
		event.Int("userId", int(bReq.UserID))
	}
	event.Send()

	servElms := strings.Split(service, "/")

	if b.esDispatcher != nil {
		rec := &storage.OnTheFlyOutputRecord{
			AppType: servElms[1],
			Rec: &ElasticOutputRecord{
				Time:           time.Now(),
				Service:        servElms[1],
				ActionType:     string(bReq.ActionType),
				ProcTime:       bReq.ProcTime,
				IsCached:       bReq.IsCached,
				FirstPartyCall: bReq.FirstPartyCall,
				UserID:         bReq.UserID,
				IPAddress:      unireq.ClientIP(req).String(),
				UserAgent:      req.UserAgent(),
				RequestPath:    strings.TrimPrefix(req.URL.Path, b.reqPathPrefix),
				Args:           exportURLArgs(req),
			},
		}
		b.esDispatcher.Send(rec)
	}
}

// NewBackendLogger creates a new backend access logging service
func NewBackendLogger(
	tDBWriter reporting.ReportingWriter,
	logPath string,
	reqPathPrefix string,
	esDispatcher *ESDispatcher,
) (*BackendLogger, error) {

	if logPath == "" {
		return &BackendLogger{
			tDBWriter:     tDBWriter,
			fileLogger:    log.Logger,
			reqPathPrefix: reqPathPrefix,
			esDispatcher:  esDispatcher,
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
		tDBWriter:     tDBWriter,
		fileLogger:    fileLogger,
		reqPathPrefix: reqPathPrefix,
		esDispatcher:  esDispatcher,
	}, nil
}
