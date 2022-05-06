// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package backend

import "wum/telemetry"

type StorageProvider interface {
	LoadTelemetry(sessionID, clientIP string, maxAgeSecs int) ([]*telemetry.ActionRecord, error)
	LoadCountingRules() ([]*telemetry.CountingRule, error)
}
