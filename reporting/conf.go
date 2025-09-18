// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Department of Linguistics,
// #              Faculty of Arts, Charles University
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
package reporting

import (
	"fmt"

	"github.com/czcorpus/hltscl"
	"github.com/rs/zerolog/log"
)

type Conf struct {
	DB hltscl.PgConf `json:"db"`
}

func (conf *Conf) ValidateAndDefaults() error {
	if conf == nil {
		log.Warn().Msg("reporting not configured, APIGuard will be writing reporting records to log")
		return nil
	}
	if conf.DB.Host == "" {
		return fmt.Errorf("reporting set but the `host` is missing")
	}
	if conf.DB.Passwd == "" {
		return fmt.Errorf("reporting set but the `password` is missing")
	}
	return nil
}
