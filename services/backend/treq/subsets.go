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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type treqResp struct {
	Sum   int   `json:"sum"`
	Lines []any `json:"lines"`
}

type subsetTreqResp struct {
	Subsets map[string]treqResp `json:"subsets"`
	Error   string              `json:"error,omitempty"`
}

func (tp *TreqProxy) Subsets(ctx *gin.Context) {

	var cached, indirectAPICall bool
	var clientID, humanID common.UserID
	t0 := time.Now().In(tp.GlobalCtx().TimezoneLocation)

	defer func(currUserID, currHumanID *common.UserID, indirect *bool, created time.Time) {
		loggedUserID := currUserID
		if currHumanID.IsValid() && tp.Guard().TestUserIsAnonymous(*currHumanID) {
			loggedUserID = currHumanID
		}
		tp.CountRequest(
			ctx,
			created,
			tp.EnvironConf().ServiceKey,
			*loggedUserID,
		)
		tp.GlobalCtx().BackendLoggers.Get(tp.EnvironConf().ServiceKey).Log(
			ctx.Request,
			tp.EnvironConf().ServiceKey,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			reporting.BackendActionTypeQuery,
		)
	}(&clientID, &humanID, &indirectAPICall, t0)

	if !strings.HasPrefix(ctx.Request.URL.Path, tp.EnvironConf().ServicePath) {
		proxy.WriteError(ctx, fmt.Errorf("invalid path detected"), http.StatusInternalServerError)
		return
	}
	reqProps := tp.Guard().EvaluateRequest(ctx.Request, tp.authFallbackCookie)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user treq/* request")
	clientID = reqProps.ClientID
	if reqProps.ProposedResponse == http.StatusUnauthorized {
		_, err, _ := tp.reauthSF.Do(
			"reauth",
			func() (any, error) {
				resp := tp.LoginWithToken(tp.conf.CNCAuthToken)
				log.Debug().Msgf("reauthentication result: %s", resp.String())
				if resp.Err() == nil {
					c := resp.Cookie(tp.EnvironConf().CNCAuthCookie)
					cVal := "-"
					if c != nil {
						cVal = c.Value
					}
					log.Debug().
						Str("serviceId", tp.EnvironConf().ServiceKey).
						Str("cookieValue", cVal).
						Msg("performed reauthentication")
					if c != nil {
						tp.authFallbackCookie = c
						return true, nil
					}
					return false, nil
				}
				return false, resp.Err()
			},
		)
		if err != nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf("failed to proxy request: %w", err),
				reqProps.ProposedResponse,
			)
			return
		}
		if tp.authFallbackCookie == nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf(
					"failed to proxy request: cnc auth cookie '%s' not found",
					tp.EnvironConf().CNCAuthCookie,
				),
				reqProps.ProposedResponse,
			)
			return
		}

	} else if reqProps.Error != nil {
		proxy.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %w", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ForbidsAccess() {
		proxy.WriteError(
			ctx,
			errors.New(http.StatusText(reqProps.ProposedResponse)),
			reqProps.ProposedResponse,
		)
		return
	}

	var allSubsetsArgs subsetsReq
	if err := ctx.BindJSON(&allSubsetsArgs); err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to decode args: %w", err), http.StatusInternalServerError)
		return
	}

	incrementalAns := make(map[string]treqResp)
	var mapLock sync.Mutex
	var wg sync.WaitGroup

	for subsetID, args := range allSubsetsArgs {
		wg.Add(1)
		go func(req http.Request, reqArgs subsetArgs) {
			defer wg.Done()

			req.Header = req.Header.Clone() // this prevents concurrent access to headers (= a map)
			if reqProps.RequiresFallbackCookie {
				log.Debug().
					Str("serviceId", tp.EnvironConf().ServiceKey).
					Str("value", tp.authFallbackCookie.Value).Msg("applying fallback cookie")
				tp.DeleteCookie(&req, tp.authFallbackCookie.Name)
				req.AddCookie(tp.authFallbackCookie)
			}
			req.Method = "GET"
			req.Body = io.NopCloser(strings.NewReader(""))
			serviceResp := tp.ProxyRequest(
				"/",
				reqArgs.ToQuery(),
				req.Method,
				req.Header,
				req.Body,
			)
			if serviceResp.Error() != nil {
				log.Error().Err(serviceResp.Error()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
				http.Error(
					ctx.Writer,
					fmt.Sprintf("failed to proxy request: %s", serviceResp.Error()),
					http.StatusInternalServerError,
				)
				return
			}

			defer serviceResp.GetBodyReader().Close()
			respBody, err := io.ReadAll(serviceResp.GetBodyReader())
			if err != nil {
				// not much we can do here
				log.Error().Err(err).Msg("failed to prepare EventSource data")
				return
			}

			var treqResp treqResp
			if err := json.Unmarshal(respBody, &treqResp); err != nil {
				// not much we can do here
				log.Error().Err(err).Msg("failed to prepare EventSource data")
				return
			}

			mapLock.Lock()
			incrementalAns[subsetID] = treqResp
			ans := subsetTreqResp{
				Subsets: incrementalAns,
			}
			rawAns, err := json.Marshal(ans)
			mapLock.Unlock()
			if err != nil {
				log.Error().Err(err).Msg("failed to prepare EventSource data")
				return
			}

			_, err = ctx.Writer.WriteString(
				fmt.Sprintf(
					"event: DataTile-%s.%d\ndata: %s\n\n", ctx.Query("tileId"), 0, rawAns),
			)
			if err != nil {
				// not much we can do here
				log.Error().Err(err).Msg("failed to write EventSource data")
				return
			}

		}(*ctx.Request, args)
	}
	wg.Wait()
	ctx.Writer.Flush()
}
