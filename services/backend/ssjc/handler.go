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

package ssjc

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "ssjc"
)

type SSJCActions struct {
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	guard           guard.ServiceGuard
}

type Entry struct {
	STI     *int   `json:"sti"`
	Payload string `json:"payload"`
}

type Response struct {
	Entries []Entry `json:"entries"`
	Query   string  `json:"query"`
}

func (aa *SSJCActions) Query(ctx *gin.Context) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		aa.globalCtx.BackendLoggers.Get(aa.serviceKey).Log(
			ctx.Request,
			ServiceName,
			time.Since(t0),
			cached,
			common.InvalidUserID,
			false,
			reporting.BackendActionTypeQuery,
		)
	}()

	queries, ok := ctx.Request.URL.Query()["q"]
	if !ok {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}
	if len(queries) != 1 && len(queries) > aa.conf.MaxQueries {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("too many queries"), 422)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard, clientID)
	if err != nil {
		return
	}

	response := Response{Entries: make([]Entry, 0)}
	var query string
	for _, query = range queries {
		resp := aa.createMainRequest(
			fmt.Sprintf("%s/search.php?hledej=Hledat&heslo=%s&where=hesla&hsubstr=no", aa.conf.BaseURL, url.QueryEscape(query)),
			ctx.Request,
		)
		cached = cached || resp.IsCacheHit()

		// check if there are multiple results
		respBody, err := resp.ExportResponse()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		STIs, err := lookForSTI(string(respBody))
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}

		if len(STIs) > 0 {
			for _, STI := range STIs {
				resp := aa.createMainRequest(
					fmt.Sprintf("%s/search.php?hledej=Hledat&sti=%d&where=hesla&hsubstr=no", aa.conf.BaseURL, STI),
					ctx.Request,
				)
				cached = cached || resp.IsCacheHit()
				payload, err := parseData(string(respBody))
				if err != nil {
					uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
					return
				}
				response.Entries = append(
					response.Entries,
					Entry{STI: &STI, Payload: payload},
				)
			}
			response.Query = query
			break
		} else {
			payload, err := parseData(string(respBody))
			if err != nil {
				uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
				return
			}
			if len(payload) > 0 {
				response.Query = query
				response.Entries = append(
					response.Entries,
					Entry{STI: nil, Payload: payload},
				)
				break
			}
		}
	}

	uniresp.WriteJSONResponse(ctx.Writer, response)
}

func (aa *SSJCActions) createMainRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, aa.conf.ClientUserAgent, aa.globalCtx.Cache)
}

func NewSSJCActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	guard guard.ServiceGuard,
	readTimeoutSecs int,
) *SSJCActions {
	return &SSJCActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
