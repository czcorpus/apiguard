// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package logging

import (
	"time"

	"github.com/rs/zerolog/log"
)

func LogServiceRequest(service string, procTime time.Duration, cached *bool, userId *int) {
	event := log.Info().
		Str("type", "apiguard").
		Str("service", service).
		Float64("procTime", procTime.Seconds()).
		Bool("isCached", *cached)
	if userId != nil {
		event.Int("userId", *userId)
	}
	event.Send()
}
