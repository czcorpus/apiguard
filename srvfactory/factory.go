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

package srvfactory

import (
	"net/http"

	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/monitoring"
	"github.com/czcorpus/apiguard/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type ServiceInitializer func(args services.InitArgs) error

var (
	serviceInitalizers = make(map[string]ServiceInitializer)
)

func RegisterServiceInitializer(name string, factory ServiceInitializer) {
	log.Info().Str("service", name).Msg("registered service initializer")
	serviceInitalizers[name] = factory
}

func InitServices(
	ctx *globctx.Context,
	engine http.Handler,
	apiRoutes *gin.RouterGroup,
	globalConf *config.Configuration,
	alarm *monitoring.AlarmTicker,
) {
	for sid, servConf := range globalConf.Services {

		initialize, ok := serviceInitalizers[servConf.Type]
		if !ok {
			log.Warn().Msgf("Ignoring unknown service %d/%s", sid, servConf.Type)
			continue
		}
		log.Info().Msgf("registering service %d/%s", sid, servConf.Type)
		if err := initialize(services.InitArgs{
			Ctx:        ctx,
			Engine:     engine,
			APIRoutes:  apiRoutes,
			GlobalConf: globalConf,
			SID:        sid,
			RawConf:    servConf.Conf,
			Alarm:      alarm,
		}); err != nil {
			log.Fatal().
				Err(err).
				Msgf(
					"failed to start APIGuard, failed to initialize service %d/%s",
					sid, servConf.Type,
				)
			return
		}
	}
}
