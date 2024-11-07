// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/config"
	"apiguard/services/cnc"
	"fmt"

	"github.com/rs/zerolog/log"
)

type Conf struct {
	cnc.ProxyConf
}

func (conf *Conf) Validate(name string) error {
	if err := conf.ProxyConf.Validate(name); err != nil {
		return err
	}
	if conf.ReqTimeoutSecs == 0 {
		conf.ReqTimeoutSecs = config.DfltProxyReqTimeoutSecs
		log.Warn().Msgf(
			"%s: missing reqTimeoutSecs, setting %d", name, config.DfltProxyReqTimeoutSecs)

	} else if conf.ReqTimeoutSecs < 0 {
		return fmt.Errorf("%s: invalid reqTimeoutSecs value: %d", name, conf.ReqTimeoutSecs)
	}
	if conf.IdleConnTimeoutSecs == 0 {
		conf.IdleConnTimeoutSecs = config.DfltIdleConnTimeoutSecs
		log.Warn().Msgf(
			"%s: missing idleConnTimeoutSecs, setting %d", name, config.DfltIdleConnTimeoutSecs)

	} else if conf.IdleConnTimeoutSecs < 0 {
		return fmt.Errorf("%s: invalid idleConnTimeoutSecs value: %d", name, conf.IdleConnTimeoutSecs)
	}
	return nil
}
