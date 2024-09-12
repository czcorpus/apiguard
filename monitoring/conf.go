// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"fmt"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/mail"
	"github.com/rs/zerolog/log"
)

const (
	DfltCleanupMaxAgeDays        = 7
	DfltUserReqCounterBufferSize = 500
	DfltExceedingsBufferSize     = 10
	DfltExceedingThreshold       = 0.05
)

type Limit struct {
	ReqPerTimeThreshold     int `json:"reqPerTimeThreshold"`
	ReqCheckingIntervalSecs int `json:"reqCheckingIntervalSecs"`
}

func (m Limit) ReqCheckingInterval() time.Duration {
	return time.Duration(m.ReqCheckingIntervalSecs) * time.Second
}

// AlarmConf describes alarm setup for a concrete service
type AlarmConf struct {
	Recipients                   []string `json:"recipients"`
	RecCounterCleanupProbability float64  `json:"recCounterCleanupProbability"`
}

type MailConf struct {
	mail.NotificationConf
}

type LimitingConf struct {
	DelayLogCleanupMaxAgeDays int     `json:"delayLogCleanupMaxAgeDays"`
	StatusDataDir             string  `json:"statusDataDir"`
	UserReqCounterBufferSize  int     `json:"userReqCounterBufferSize"`
	ExceedingsBufferSize      int     `json:"exceedingsBufferSize"`
	ExceedingThreshold        float64 `json:"exceedingThreshold"`
}

func (lconf *LimitingConf) ValidateAndDefaults() error {
	if lconf == nil {
		return fmt.Errorf("missing the `limiting` section")
	}
	if lconf.DelayLogCleanupMaxAgeDays == 0 {
		lconf.DelayLogCleanupMaxAgeDays = DfltCleanupMaxAgeDays
		log.Warn().
			Int("value", DfltCleanupMaxAgeDays).
			Msg("limiting.delayLogCleanupMaxAgeDays not set, using default")

	} else if lconf.DelayLogCleanupMaxAgeDays < 0 || lconf.DelayLogCleanupMaxAgeDays > 730 {
		return fmt.Errorf("limiting.delayLogCleanupMaxAgeDays must be between 1 and 730")
	}

	if lconf.UserReqCounterBufferSize == 0 {
		lconf.UserReqCounterBufferSize = DfltUserReqCounterBufferSize
		log.Warn().
			Int("value", DfltUserReqCounterBufferSize).
			Msg("limiting.userReqCounterBufferSize not set, using default")

	} else if lconf.UserReqCounterBufferSize < 0 {
		return fmt.Errorf("limiting.userReqCounterBufferSize has an invalid value")
	}

	if lconf.ExceedingsBufferSize == 0 {
		lconf.ExceedingsBufferSize = DfltExceedingsBufferSize
		log.Warn().
			Int("value", DfltExceedingsBufferSize).
			Msg("limiting.exceedingsBufferSize not set, using default")

	} else if lconf.ExceedingsBufferSize < 0 {
		return fmt.Errorf("limiting.exceedingsBufferSize has an invalid value")
	}

	if lconf.ExceedingThreshold == 0 {
		lconf.ExceedingThreshold = DfltExceedingThreshold
		log.Warn().
			Float64("value", DfltExceedingThreshold).
			Msg("limiting.exceedingThreshold not set, using default")

	} else if lconf.ExceedingThreshold < 0 || lconf.ExceedingThreshold >= 1 {
		return fmt.Errorf("limiting.exceedingThreshold has an invalid value - must be between 0 and 1 (excluding)")
	}

	isDir, err := fs.IsDir(lconf.StatusDataDir)
	if err != nil {
		return fmt.Errorf("failed to test limiting.statusDataDir: %w", err)
	}
	if !isDir {
		return fmt.Errorf(
			"invalid limiting.statusDataDir - not a directory: %s", lconf.StatusDataDir)
	}
	return nil
}
