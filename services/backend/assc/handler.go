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

package assc

import (
	"fmt"
	"net/url"
	"time"

	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-common/guard"
	"github.com/czcorpus/apiguard-common/reporting"
	"github.com/czcorpus/apiguard/proxy"

	guardImpl "github.com/czcorpus/apiguard/guard"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

/*
note:
curl
	--header "Origin: https://slovnikcestiny.cz"
	--header "Referer: https://slovnikcestiny.cz/heslo/batalion/0/548"
	--header "X-Requested-With: XMLHttpRequest" --header "Host: slovnikcestiny.cz"
	https://slovnikcestiny.cz/web_ver_ajax.php
*/

const (
	ServiceName = "assc"
)

type ASSCActions struct {
	globalCtx       *globctx.Context
	conf            *Conf
	readTimeoutSecs int
	serviceKey      string
	guard           guard.ServiceGuard
}

func (aa *ASSCActions) Query(ctx *gin.Context) {
	var cached bool
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	defer func() {
		aa.globalCtx.BackendLoggers[aa.serviceKey].Log(
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
	err := guardImpl.RestrictResponseTime(
		ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard, clientID)
	if err != nil {
		return
	}

	var data *dataStruct
	for _, query := range queries {
		response := proxy.UJCGetRequest(
			fmt.Sprintf("%s/heslo/%s/", aa.conf.BaseURL, url.QueryEscape(query)),
			aa.conf.ClientUserAgent,
			aa.globalCtx.Cache,
		)
		cached = cached || response.IsCacheHit()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		respBody, err := response.ExportResponse()
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		data, err = parseData(string(respBody))
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
			return
		}
		// check if result is not empty and contains query key
		if data.lastItem != nil {
			data.Query = query
			break
		}
	}
	uniresp.WriteJSONResponse(ctx.Writer, data)
}

func NewASSCActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	guard guard.ServiceGuard,
	readTimeoutSecs int,
) *ASSCActions {
	return &ASSCActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
