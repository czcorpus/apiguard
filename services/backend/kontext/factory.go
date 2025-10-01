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

package kontext

import (
	"encoding/json"
	"fmt"
	"net/http"

	iGuard "github.com/czcorpus/apiguard-common/guard"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/cncauth"
	"github.com/czcorpus/apiguard/guard/null"
	"github.com/czcorpus/apiguard/services"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/apiguard/session"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func init() {
	srvfactory.RegisterServiceInitializer("kontext", create)
}

func create(args services.InitArgs) error {
	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (kontext): %w", args.SID, err)
	}
	if err := typedConf.Validate(fmt.Sprintf("%d/kontext", args.SID)); err != nil {
		return fmt.Errorf("failed to initialize service %d (kontext): %w", args.SID, err)
	}
	var cncGuard iGuard.ServiceGuard
	switch typedConf.SessionValType {
	case session.SessionTypeNone:
		cncGuard = &null.Guard{}
		log.Warn().
			Msgf("service %d/kontext is running in 'null' session mode - this is mostly for testing", args.SID)
	case session.SessionTypeCNC:
		cncGuard = cncauth.New(
			args.Ctx,
			args.GlobalConf.CNCAuth.SessionCookieName,
			typedConf.FrontendSessionCookieName,
			typedConf.SessionValType,
			typedConf.Limits,
		)
	default:
		return fmt.Errorf(
			"failed to initialize service %d/kontext: service does not support session type %s",
			args.SID, typedConf.SessionValType,
		)
	}

	var kontextReqCounter chan<- guard.RequestInfo
	if len(typedConf.Limits) > 0 {
		kontextReqCounter = args.Alarm.Register(
			fmt.Sprintf("%d/kontext", args.SID),
			typedConf.Alarm,
			typedConf.Limits,
		)
	}
	kontextActions, err := NewKontextProxy(
		args.Ctx,
		&typedConf,
		&cnc.EnvironConf{
			CNCAuthCookie:     args.GlobalConf.CNCAuth.SessionCookieName,
			AuthTokenEntry:    cnc.AuthTokenEntry,
			ServicePath:       fmt.Sprintf("/service/%d/kontext", args.SID),
			ServiceKey:        fmt.Sprintf("%d/kontext", args.SID),
			CNCPortalLoginURL: cnc.PortalLoginURL,
			ReadTimeoutSecs:   args.GlobalConf.ServerReadTimeoutSecs,
			IsStreamingMode:   args.GlobalConf.OperationMode == config.OperationModeStreaming,
		},
		cncGuard,
		kontextReqCounter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (kontext): %w", args.SID, err)
	}

	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/kontext/*path", args.SID),
		func(ctx *gin.Context) {
			if ctx.Param("path") == "/login" && ctx.Request.Method == http.MethodPost {
				kontextActions.Login(ctx)

			} else if ctx.Param("path") == "/preflight" {
				kontextActions.Preflight(ctx)

			} else if ctx.Param("path") == "/query_submit" {
				kontextActions.QuerySubmitAndView(ctx)

			} else {
				kontextActions.AnyPath(ctx)
			}
		},
	)
	log.Info().Int("sid", args.SID).Msg("Proxy for Kontext enabled")
	return nil
}
