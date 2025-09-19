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

package cja

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ServiceName = "cja"
)

type CJAActions struct {
	globalCtx       *globctx.Context
	serviceKey      string
	conf            *Conf
	readTimeoutSecs int
	guard           guard.ServiceGuard
}

type Response struct {
	Content  string `json:"content"`
	Image    string `json:"image"`
	CSS      string `json:"css"`
	Backlink string `json:"backlink"`
}

func (aa *CJAActions) Query(ctx *gin.Context) {
	t0 := time.Now().In(aa.globalCtx.TimezoneLocation)
	var cached bool
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

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: common.InvalidUserID,
	}
	err := guard.RestrictResponseTime(ctx.Writer, ctx.Request, aa.readTimeoutSecs, aa.guard, clientID)
	if err != nil {
		return
	}
	resp := aa.createRequests(
		fmt.Sprintf("%s/e-cja/h/?hw=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		fmt.Sprintf("%s/e-cja/h/?doklad=%s", aa.conf.BaseURL, url.QueryEscape(query)),
		ctx.Request,
	)
	if resp.Error() != nil {
		uniresp.RespondWithErrorJSON(ctx, resp.Error(), 500)
		return
	}
	respBody, err := resp.ExportResponse()
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, 500)
		return
	}
	response, err := parseData(string(respBody), aa.conf.BaseURL)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, 500)
		return
	}
	// TODO !!!! response.Backlink = backlink

	uniresp.WriteJSONResponse(ctx.Writer, response)
}

func (aa *CJAActions) createSubRequest(url string, req *http.Request) proxy.ResponseProcessor {
	return proxy.UJCGetRequest(url, req.Header.Get("user-agent"), aa.globalCtx.Cache)
}

func (aa *CJAActions) createRequests(url1 string, url2 string, req *http.Request) proxy.ResponseProcessor {
	resp := aa.createSubRequest(url1, req)
	if resp.Error() != nil {
		return resp
	}
	if resp.Response().GetStatusCode() == 500 {
		return aa.createSubRequest(url2, req)
	}
	return resp
}

func NewCJAActions(
	globalCtx *globctx.Context,
	serviceKey string,
	conf *Conf,
	guard guard.ServiceGuard,
	readTimeoutSecs int,
) *CJAActions {
	return &CJAActions{
		globalCtx:       globalCtx,
		serviceKey:      serviceKey,
		conf:            conf,
		guard:           guard,
		readTimeoutSecs: readTimeoutSecs,
	}
}
