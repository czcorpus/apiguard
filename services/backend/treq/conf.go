// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/proxy"
	"apiguard/services/cnc"
	"fmt"

	"github.com/rs/zerolog/log"
)

const (
	defaultNumExamplesPerWord = 1
)

type Conf struct {
	cnc.ProxyConf
	CNCAuthToken          string `json:"cncAuthToken"`
	ConcMQueryServicePath string `json:"concMqueryServicePath"`
	NumExamplesPerWord    int    `json:"numExamplesPerWord"`
}

func (c *Conf) Validate(context string) error {
	if c.BackendURL == "" {
		return fmt.Errorf("%s.backendUrl is missing/empty", context)
	}
	if err := c.SessionValType.Validate(); err != nil {
		return fmt.Errorf("%s.sessionValType is invalid: %w", context, err)
	}
	if c.NumExamplesPerWord == 0 {
		log.Warn().
			Int("default", defaultNumExamplesPerWord).
			Msg("service.treq.numExamplesPerWord not set, using default")
		c.NumExamplesPerWord = defaultNumExamplesPerWord
	}
	return nil
}

func (c *Conf) GetCoreConf() proxy.GeneralProxyConf {
	return proxy.GeneralProxyConf{
		BackendURL:          c.BackendURL,
		FrontendURL:         c.FrontendURL,
		ReqTimeoutSecs:      c.ReqTimeoutSecs,
		IdleConnTimeoutSecs: c.IdleConnTimeoutSecs,
		Limits:              c.Limits,
	}
}
