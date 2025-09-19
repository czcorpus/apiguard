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

package mquery

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/reporting"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type concordanceRequestArgs struct {
	Q             string
	Subcorpus     string
	Format        string
	ShowMarkup    bool
	ShowTextProps bool
	ContextWidth  int
	Coll          string
	CollRange     string
}

func (cra *concordanceRequestArgs) ToURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", cra.Q)
	if cra.Subcorpus != "" {
		q.Add("subcorpus", cra.Subcorpus)
	}
	if cra.Format != "" {
		q.Add("format", cra.Format)
	}
	if cra.ShowMarkup {
		q.Add("showMarkup", "1")
	}
	if cra.ShowTextProps {
		q.Add("showTextProps", "1")
	}
	q.Add("contextWidth", fmt.Sprint(cra.ContextWidth))
	if cra.Coll != "" {
		q.Add("coll", cra.Coll)
	}
	if cra.CollRange != "" {
		q.Add("collRange", cra.CollRange)
	}
	return q.Encode()
}

type tokenContextRequestArgs struct {
	Pos      int
	LeftCtx  int
	RightCtx int
	Attrs    []string
	Structs  []string
}

func (tcra *tokenContextRequestArgs) ToURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("idx", fmt.Sprint(tcra.Pos))
	q.Add("leftCtx", fmt.Sprint(tcra.LeftCtx))
	q.Add("rightCtx", fmt.Sprint(tcra.RightCtx))
	for _, attr := range tcra.Attrs {
		q.Add("attr", attr)
	}
	for _, struc := range tcra.Structs {
		q.Add("struct", struc)
	}
	return q.Encode()
}

// --------------------------------------

type concordanceToken struct {
	Type   string            `json:"type"`
	Word   string            `json:"word"`
	Strong bool              `json:"strong"`
	Attrs  map[string]string `json:"attrs"`
}

// --------------------------------------

type concordanceLine struct {
	Text []concordanceToken `json:"text"`
	Ref  string             `json:"ref"`
}

// --------------------------------------

type concordanceResponse struct {
	Lines      []concordanceLine `json:"lines"`
	ConcSize   int               `json:"concSize"`
	ResultType string            `json:"resultType"`
	Error      string            `json:"error,omitempty"`
}

// ------------------------------

func (mp *MQueryProxy) createConcMqueryURL(
	corpusID string,
	reqArgs concordanceRequestArgs,
) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(
		mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "concordance", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create concordance URL: %w", err)
	}
	url2.RawQuery = reqArgs.ToURLQuery()
	return url2, nil
}

func (mp *MQueryProxy) createTokenContextMqueryURL(
	corpusID string,
	reqArgs tokenContextRequestArgs,
) (*url.URL, error) {
	rawUrl2, err := url.JoinPath(
		mp.Proxy.BackendURL.String(), mp.EnvironConf().ServicePath, "token-context", corpusID)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create token-context URL: %w", err)
	}
	url2, err := url.Parse(rawUrl2)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to create token-context URL: %w", err)
	}
	url2.RawQuery = reqArgs.ToURLQuery()
	return url2, nil
}

func (mp *MQueryProxy) Speeches(ctx *gin.Context) {
	var userID, humanID common.UserID
	var cached, indirectAPICall bool
	var statusCode int
	t0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)

	defer mp.LogRequest(ctx, &humanID, &indirectAPICall, &cached, t0)

	if !strings.HasPrefix(ctx.Request.URL.Path, mp.EnvironConf().ServicePath) {
		log.Error().Msgf("failed to get speeches - invalid path detected")
		http.Error(ctx.Writer, "Invalid path detected", http.StatusInternalServerError)
		return
	}
	reqProps, ok := mp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err := mp.Guard().DetermineTrueUserID(ctx.Request)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract human user ID information")
		http.Error(ctx.Writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if humanID == common.InvalidUserID {
		humanID = reqProps.ClientID
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: humanID,
	}

	if err := guard.RestrictResponseTime(
		ctx.Writer, ctx.Request, mp.EnvironConf().ReadTimeoutSecs, mp.Guard(), clientID,
	); err != nil {
		return
	}

	if err := mp.ProcessReqHeaders(
		ctx, humanID, userID, &indirectAPICall,
	); err != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to get speeches - cookie mapping")
		http.Error(
			ctx.Writer,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rt0 := time.Now().In(mp.GlobalCtx().TimezoneLocation)
	args := speechesArgs{
		Corpname:  ctx.Request.URL.Query().Get("corpname"),
		Subcorpus: ctx.Request.URL.Query().Get("subcorpus"),
		Query:     ctx.Request.URL.Query().Get("query"),
	}
	if leftCtx := ctx.Request.URL.Query().Get("leftCtx"); leftCtx != "" {
		if i, err := strconv.Atoi(leftCtx); err == nil {
			args.LeftCtx = i
		} else {
			uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Errorf("failed to parse leftCtx: %s", err), http.StatusBadRequest)
			return
		}
	}
	if rightCtx := ctx.Request.URL.Query().Get("rightCtx"); rightCtx != "" {
		if i, err := strconv.Atoi(rightCtx); err == nil {
			args.RightCtx = i
		} else {
			uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Errorf("failed to parse rightCtx: %s", err), http.StatusBadRequest)
			return
		}
	}
	if structs, ok := ctx.Request.URL.Query()["struct"]; ok {
		args.Structs = structs
	}

	req1URL, err := mp.createConcMqueryURL(
		args.Corpname,
		concordanceRequestArgs{
			Q:            args.Query,
			Subcorpus:    args.Subcorpus,
			ContextWidth: 0,
			CollRange:    "0,0",
		},
	)
	if err != nil {
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, fmt.Sprintf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	req1 := *ctx.Request
	req1.URL = req1URL
	req1.Method = "GET"
	serviceResp := mp.HandleRequest(&req1, reqProps, true)
	statusCode = serviceResp.Response().GetStatusCode()
	if err := serviceResp.Error(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, statusCode)
		return
	}
	resp1Body, err := serviceResp.ExportResponse()
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	var resp1 concordanceResponse
	if err := sonic.Unmarshal(resp1Body, &resp1); err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	if resp1.Error != "" {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf(resp1.Error), statusCode)
		return
	}

	if len(resp1.Lines) == 0 {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("no data found"), http.StatusNotFound)
		return
	}

	pos, err := strconv.Atoi(resp1.Lines[rand.Intn(len(resp1.Lines))].Ref[1:])
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("failed to query concordance: %s", err), http.StatusInternalServerError)
		return
	}
	req2URL, err := mp.createTokenContextMqueryURL(
		args.Corpname,
		tokenContextRequestArgs{
			Pos:      pos,
			LeftCtx:  args.LeftCtx,
			RightCtx: args.RightCtx,
			Structs:  args.Structs,
		},
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("failed to query token context: %s", err), http.StatusInternalServerError)
		return
	}
	req2 := *ctx.Request
	req2.URL = req2URL
	req2.Method = "GET"
	serviceResp = mp.HandleRequest(&req2, reqProps, true)
	statusCode = serviceResp.Response().GetStatusCode()
	if err := serviceResp.Error(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, statusCode)
		return
	}

	ctx.Status(serviceResp.Response().GetStatusCode())
	respBody, err := serviceResp.ExportResponse()
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, statusCode)
		return
	}
	ctx.Writer.Write(respBody)

	mp.MonitoringWrite(&reporting.ProxyProcReport{
		DateTime: time.Now().In(mp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   statusCode,
		Service:  mp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
}
