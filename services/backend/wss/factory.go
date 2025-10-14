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

package wss

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	iGuard "github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/dflt"
	"github.com/czcorpus/apiguard/guard/token"
	"github.com/czcorpus/apiguard/services"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func init() {
	srvfactory.RegisterServiceInitializer("wss", create)
}

func create(args services.InitArgs) error {
	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (wss): %w", args.SID, err)
	}
	if err := typedConf.Validate("wss"); err != nil {
		return fmt.Errorf("failed to initialize service %d (wss): %w", args.SID, err)
	}
	analyzer := dflt.New(
		args.Ctx,
		args.GlobalConf.CNCAuth.SessionCookieName,
		typedConf.SessionValType,
		typedConf.Limits,
	)
	go analyzer.Run()

	var wssReqCounter chan<- guard.RequestInfo
	if len(typedConf.Limits) > 0 {
		wssReqCounter = args.Alarm.Register(
			fmt.Sprintf("%d/wss", args.SID),
			typedConf.Alarm,
			typedConf.Limits,
		)
	}

	var grd iGuard.ServiceGuard
	switch typedConf.GuardType {
	case guard.GuardTypeToken:
		grd = token.NewGuard(
			args.Ctx,
			fmt.Sprintf("/service/%d/wss", args.SID),
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
		return fmt.Errorf("MQuery proxy does not support guard type `%s`", typedConf.GuardType)
	}

	wssActions, err := NewWSServerProxy(
		args.Ctx,
		&typedConf.ProxyConf,
		&cnc.EnvironConf{
			CNCAuthCookie:     args.GlobalConf.CNCAuth.SessionCookieName,
			AuthTokenEntry:    cnc.AuthTokenEntry,
			ServicePath:       fmt.Sprintf("/service/%d/wss", args.SID),
			ServiceKey:        fmt.Sprintf("%d/wss", args.SID),
			CNCPortalLoginURL: cnc.PortalLoginURL,
			ReadTimeoutSecs:   args.GlobalConf.ServerReadTimeoutSecs,
			IsStreamingMode:   args.GlobalConf.OperationMode == config.OperationModeStreaming,
		},
		grd,
		args.Engine,
		wssReqCounter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (wss): %w", args.SID, err)
	}

	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/wss/*path", args.SID),
		func(ctx *gin.Context) {
			if ctx.Param("path") == "/collocations-tt" && ctx.Request.Method == http.MethodPost {
				wssActions.CollocationsTT(ctx)

			} else {
				wssActions.AnyPath(ctx)
			}
		},
	)
	log.Info().Int("args.SID", args.SID).Msg("Proxy for wss enabled")
	return nil
}
