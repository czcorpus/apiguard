// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/guard"
	"apiguard/monitoring"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

func runCleanup(db *sql.DB, loc *time.Location, conf *monitoring.LimitingConf) {
	log.Info().Msg("running cleanup procedure")
	delayLog := guard.NewDelayStats(db, loc)
	ans := delayLog.CleanOldData(conf.DelayLogCleanupMaxAgeDays)
	if ans.Error != nil {
		log.Fatal().Err(ans.Error).Msg("failed to cleanup old records")
	}
	status, err := json.Marshal(ans)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to provide cleanup summary")
	}
	log.Info().Msgf("finished old data cleanup: %s", string(status))
}
