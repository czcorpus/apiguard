// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
