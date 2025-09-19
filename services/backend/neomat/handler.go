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

package neomat

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

const (
	ServiceName = "neomat"
)

type NeomatActions struct {
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	analyzer        guard.ServiceGuard
}

type Response struct {
	Entries []string `json:"entries"`
}

func (aa *NeomatActions) Query(ctx *gin.Context) {
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

	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty query"), 422)
		return
	}
	maxItems := ctx.Request.URL.Query().Get("maxItems")
	if maxItems == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("empty maxItems"), 422)
		return
	}
	maxItemsCount, err := strconv.Atoi(maxItems)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err = guardImpl.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.analyzer, clientID)
	if err != nil {
		return
	}
	resp := aa.createMainRequest(
		fmt.Sprintf("%s/index.php?retezec=%s&prijimam=1", aa.conf.BaseURL, url.QueryEscape(query)),
		ctx.Request,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}
	entries, err := parseData(string(respBody), maxItemsCount)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
		return
	}

	uniresp.WriteJSONResponse(ctx.Writer, Response{Entries: entries})
}

func (aa *NeomatActions) createMainRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, aa.conf.ClientUserAgent, aa.globalCtx.Cache)
}

func NewNeomatActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	analyzer guard.ServiceGuard,
	readTimeoutSecs int,
) *NeomatActions {
	return &NeomatActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		analyzer:        analyzer,
		readTimeoutSecs: readTimeoutSecs,
	}
}
