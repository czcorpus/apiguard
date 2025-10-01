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

package scollex

import (
	"encoding/json"
	"fmt"
	"net/http"

	iGuard "github.com/czcorpus/apiguard-common/guard"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/dflt"
	"github.com/czcorpus/apiguard/guard/token"
	"github.com/czcorpus/apiguard/services"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/apiguard/session"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func init() {
	srvfactory.RegisterServiceInitializer("scollex", create)
}

func create(args services.InitArgs) error {
	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (scollex): %w", args.SID, err)
	}
	// we don't want to bother admins with none session type, so we set it here
	typedConf.SessionValType = session.SessionTypeNone
	if err := typedConf.Validate("scollex"); err != nil {
		return fmt.Errorf("failed to initialize service %d (scollex): %w", args.SID, err)
	}
	var scollexReqCounter chan<- guard.RequestInfo
	if len(typedConf.Limits) > 0 {
		scollexReqCounter = args.Alarm.Register(
			fmt.Sprintf("%d/scollex", args.SID),
			typedConf.Alarm,
			typedConf.Limits,
		)
	}
	var grd iGuard.ServiceGuard
	switch typedConf.GuardType {
	case guard.GuardTypeToken:
		grd = token.NewGuard(
			args.Ctx,
			fmt.Sprintf("/service/%d/scollex", args.SID),
			typedConf.TokenHeaderName,
			typedConf.Limits,
			typedConf.Tokens,
			[]string{"/openapi"},
		)
	case guard.GuardTypeDflt:
		grd = dflt.New(
			args.Ctx,
			args.GlobalConf.CNCAuth.SessionCookieName,
			typedConf.SessionValType,
			typedConf.Limits,
		)
	default:
		return fmt.Errorf("scollex proxy does not support guard type `%s`", typedConf.GuardType)
	}

	scollexActions, err := NewScollexProxy(
		args.Ctx,
		&typedConf.ProxyConf,
		&cnc.EnvironConf{
			CNCAuthCookie:     args.GlobalConf.CNCAuth.SessionCookieName,
			AuthTokenEntry:    cnc.AuthTokenEntry,
			ServicePath:       fmt.Sprintf("/service/%d/scollex", args.SID),
			ServiceKey:        fmt.Sprintf("%d/scollex", args.SID),
			CNCPortalLoginURL: cnc.PortalLoginURL,
			ReadTimeoutSecs:   args.GlobalConf.ServerReadTimeoutSecs,
			IsStreamingMode:   args.GlobalConf.OperationMode == config.OperationModeStreaming,
		},
		grd,
		scollexReqCounter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (scollex): %w", args.SID, err)
	}

	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/scollex/*path", args.SID),
		func(ctx *gin.Context) {
			if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
				scollexActions.Login(ctx)

			} else {
				scollexActions.AnyPath(ctx)
			}
		},
	)
	log.Info().Int("args.SID", args.SID).Msg("Proxy for Scollex enabled")
	return nil
}
