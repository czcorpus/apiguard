// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

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
