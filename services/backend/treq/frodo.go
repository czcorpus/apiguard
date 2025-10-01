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

package treq

import (
	"encoding/json"
	"fmt"

	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/cncauth"
	"github.com/czcorpus/apiguard/services"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func init() {
	srvfactory.RegisterServiceInitializer("treq", create)
}

func create(args services.InitArgs) error {
	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (treq): %w", args.SID, err)
	}
	if err := typedConf.Validate("treq"); err != nil {
		return fmt.Errorf("failed to initialize service %d (treq): %w", args.SID, err)
	}
	cnca := cncauth.New(
		args.Ctx,
		args.GlobalConf.CNCAuth.SessionCookieName,
		typedConf.FrontendSessionCookieName,
		typedConf.SessionValType,
		typedConf.Limits,
	)
	var treqReqCounter chan<- guard.RequestInfo
	if len(typedConf.Limits) > 0 {
		treqReqCounter = args.Alarm.Register(
			fmt.Sprintf("%d/treq", args.SID), typedConf.Alarm, typedConf.Limits)
	}
	treqActions, err := NewTreqProxy(
		args.Ctx,
		&typedConf,
		&cnc.EnvironConf{
			CNCAuthCookie:     args.GlobalConf.CNCAuth.SessionCookieName,
			AuthTokenEntry:    cnc.AuthTokenEntry,
			ServicePath:       fmt.Sprintf("/service/%d/treq", args.SID),
			ServiceKey:        fmt.Sprintf("%d/treq", args.SID),
			CNCPortalLoginURL: cnc.PortalLoginURL,
			ReadTimeoutSecs:   args.GlobalConf.ServerReadTimeoutSecs,
			IsStreamingMode:   args.GlobalConf.OperationMode == config.OperationModeStreaming,
		},
		cnca,
		args.Engine,
		treqReqCounter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (treq): %w", args.SID, err)
	}
	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/treq/*path", args.SID),
		func(ctx *gin.Context) {
			if ctx.Param("path") == "/api/v1/subsets" {
				treqActions.Subsets(ctx)

			} else if ctx.Param("path") == "/api/v1/with-examples" {
				treqActions.WithExamples(ctx)

			} else {
				treqActions.AnyPath(ctx)
			}
		},
	)
	log.Info().Int("args.SID", args.SID).Msg("Proxy for Treq enabled")
	return nil
}
