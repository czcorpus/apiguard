// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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

//go:build !closed

package ujc

import (
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func InitUJCService(
	ctx *globctx.Context,
	sid int,
	servConf config.GeneralServiceConf,
	globalConf *config.Configuration,
	apiRoutes gin.IRoutes,
) error {
	log.Warn().Msgf("Ignoring closed-source UJC service %d: %s", sid, servConf.Type)
	return nil
}