// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package backend

import (
	"errors"
	"wum/telemetry"
)

type TelemetryStorage interface {
	LoadClientTelemetry(sessionID, clientIP string, maxAgeSecs, minAgeSecs int) ([]*telemetry.ActionRecord, error)
	FindLearningClients(maxAgeSecs, minAgeSecs int) ([]*telemetry.Client, error)
	LoadCountingRules() ([]*telemetry.CountingRule, error)
}

var ErrUnknownClient = errors.New("unknown client")
