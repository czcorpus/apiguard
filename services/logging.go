// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"time"

	"github.com/rs/zerolog/log"
)

func LogEvent(service string, t0 time.Time, msg string) {
	event := log.Info().
		Str("type", "apiguard").
		Str("service", service).
		Float64("procTime", float64(time.Since(t0)))

	if len(msg) > 0 {
		event.Msg(msg)
	}
	event.Send()
}
