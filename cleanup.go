// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/cnc/guard"
	"apiguard/config"
	"database/sql"
	"encoding/json"

	"github.com/rs/zerolog/log"
)

func runCleanup(db *sql.DB, conf *config.Configuration) {
	log.Info().Msg("running cleanup procedure")
	delayLog := guard.NewDelayStats(db, conf.TimezoneLocation())
	ans := delayLog.CleanOldData(conf.CleanupMaxAgeDays)
	if ans.Error != nil {
		log.Fatal().Err(ans.Error).Msg("failed to cleanup old records")
	}
	status, err := json.Marshal(ans)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to provide cleanup summary")
	}
	log.Info().Msgf("finished old data cleanup: %s", string(status))
}
