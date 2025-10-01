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

package mquery

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
	srvfactory.RegisterServiceInitializer("mquery", create)
}

func afterHandlerCallback(callback func(c *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Code executed before the handler (if needed)
		c.Next() // Process the request by calling subsequent handlers
		// Code executed after the handler finishes
		callback(c)
	}
}

func create(args services.InitArgs) error {

	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (mquery): %w", args.SID, err)
	}
	// we don't want to bother admins with none session type, so we set it here
	typedConf.SessionValType = session.SessionTypeNone
	if err := typedConf.Validate("mquery"); err != nil {
		return fmt.Errorf("failed to initialize service %d (mquery): %w", args.SID, err)
	}
	var mqueryReqCounter chan<- guard.RequestInfo
	if len(typedConf.Limits) > 0 {
		mqueryReqCounter = args.Alarm.Register(
			fmt.Sprintf("%d/mquery", args.SID),
			typedConf.Alarm,
			typedConf.Limits,
		)
	}
	var grd iGuard.ServiceGuard
	switch typedConf.GuardType {
	case guard.GuardTypeToken:
		grd = token.NewGuard(
			args.Ctx,
			fmt.Sprintf("/service/%d/mquery", args.SID),
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

	mqueryActions, err := NewMQueryProxy(
		args.Ctx,
		&typedConf.ProxyConf,
		&cnc.EnvironConf{
			CNCAuthCookie:     args.GlobalConf.CNCAuth.SessionCookieName,
			AuthTokenEntry:    cnc.AuthTokenEntry,
			ServicePath:       fmt.Sprintf("/service/%d/mquery", args.SID),
			ServiceKey:        fmt.Sprintf("%d/mquery", args.SID),
			CNCPortalLoginURL: cnc.PortalLoginURL,
			ReadTimeoutSecs:   args.GlobalConf.ServerReadTimeoutSecs,
			IsStreamingMode:   args.GlobalConf.OperationMode == config.OperationModeStreaming,
		},
		grd,
		mqueryReqCounter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (mquery): %w", args.SID, err)
	}

	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/mquery/*path", args.SID),
		afterHandlerCallback(func(ctx *gin.Context) {
			ctx.Writer.Flush() // this is important for data streaming mode
			// when we use custom writer and this call closes channel which
			// signals that a sub-stream is finished
		}),
		func(ctx *gin.Context) {
			if ctx.Param("path") == "login" && ctx.Request.Method == http.MethodPost {
				mqueryActions.Login(ctx)

			} else if ctx.Param("path") == "/preflight" {
				mqueryActions.Preflight(ctx)

			} else if ctx.Param("path") == "/merge-freqs" {
				mqueryActions.MergeFreqs(ctx)

			} else if ctx.Param("path") == "/speeches" {
				mqueryActions.Speeches(ctx)

			} else if ctx.Param("path") == "/time-dist-word" {
				mqueryActions.TimeDistAltWord(ctx)

			} else {
				mqueryActions.AnyPath(ctx)
			}
		},
	)
	log.Info().Int("args.SID", args.SID).Msg("Proxy for MQuery enabled")

	return nil
}
