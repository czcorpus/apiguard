// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reporting

import "github.com/rs/zerolog/log"

type NullWriter struct {
}

func (sw *NullWriter) LogErrors() {
	log.Info().
		Bool("fallbackReporting", true).
		Msg("NullWriter.LogErrors()")
}

func (sw *NullWriter) Write(item Timescalable) {
	log.Info().
		Bool("fallbackReporting", true).
		Any("record", item)
}

func (sw *NullWriter) AddTableWriter(tableName string) {
	log.Info().
		Bool("fallbackReporting", true).
		Msgf("NullWriter.AddTableWriter(%s)", tableName)
}
