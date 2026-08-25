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

package gramatikat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	guardImpl "github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/services/cnc"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

type lemmaProfileArgs struct {
	Lemma        string   `json:"lemma"`
	PoS          string   `json:"pos"`
	CatComb      []string `json:"catComb"`
	FrameCatComb []string `json:"frameCatComb,omitempty"`
	Corpus       string   `json:"corpus"`
}

type posReqArgs struct {
	PoS          string   `json:"pos"`
	CatComb      []string `json:"catComb"`
	FrameCatComb []string `json:"frameCatComb,omitempty"`
	Corpus       string   `json:"corpus"`
	BinCount     int      `json:"bin_count"`
}

type profileResponse struct {
	LemmaInfo json.RawMessage `json:"lemmaInfo"`
	PoSInfo   json.RawMessage `json:"posInfo"`
	Code      int             `json:"code"`
	Error     string          `json:"error,omitempty"`
}

type GramatikatProxy struct {
	*cnc.Proxy
}

func (gp *GramatikatProxy) LemmaProfile(ctx *gin.Context) {
	var /*  userID, */ humanID common.UserID
	//var cached, firstPartyAPICall bool
	//t0 := time.Now().In(gp.GlobalCtx().TimezoneLocation)
	reqProps, ok := gp.AuthorizeRequestOrRespondErr(ctx)
	if !ok {
		return
	}

	humanID, err := gp.Guard().DetermineTrueUserID(ctx.Request)
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

	reqBody, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	var reqArgs lemmaProfileArgs
	if err := json.Unmarshal(reqBody, &reqArgs); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	log.Debug().Any("reqArgs", reqArgs).Str("method", ctx.Request.Method).Msg("obtained lemma-profile args")

	if err := guard.RestrictResponseTime(
		ctx.Writer, ctx.Request, gp.EnvironConf().ReadTimeoutSecs, gp.Guard(), clientID,
	); err != nil {
		return
	}

	wg, _ := errgroup.WithContext(ctx) // TODO ctx
	var resp1Body, resp2Body []byte
	var resp1Err, resp2Err string
	var resp1Code, resp2Code int

	wg.Go(func() error {
		reqURLStr, err := url.JoinPath(gp.EnvironConf().ServicePath, "lemma")
		if err != nil {
			return err
		}
		req1URL, err := url.Parse(reqURLStr)
		if err != nil {
			return err
		}

		req := *ctx.Request
		req.URL = req1URL
		req.Method = http.MethodPost
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		serviceResp := gp.MakeCacheablePOSTRequest(&req, reqProps, reqBody)
		if err := serviceResp.Error(); err != nil {
			return err
		}
		resp1Body, err = serviceResp.ExportResponse()
		resp1Code = serviceResp.Response().GetStatusCode()
		if resp1Code >= 400 && resp1Code < 600 {
			resp1Err = http.StatusText(resp1Code)
		}
		if err != nil {
			return err
		}
		return nil
	})

	wg.Go(func() error {
		reqURLStr, err := url.JoinPath(gp.EnvironConf().ServicePath, "pos", "summary")
		if err != nil {
			return err
		}
		req1URL, err := url.Parse(reqURLStr)
		if err != nil {
			return err
		}

		reqArgsJson, err := json.Marshal(posReqArgs{
			PoS:          reqArgs.PoS,
			CatComb:      reqArgs.CatComb,
			FrameCatComb: reqArgs.FrameCatComb,
			Corpus:       reqArgs.Corpus,
			BinCount:     20,
		})

		req := *ctx.Request
		req.URL = req1URL
		req.Method = http.MethodPost
		req.Body = io.NopCloser(bytes.NewBuffer(reqArgsJson))
		serviceResp := gp.MakeCacheablePOSTRequest(&req, reqProps, reqArgsJson)
		resp1Code = serviceResp.Response().GetStatusCode()
		if resp1Code >= 400 && resp1Code < 600 {
			resp2Err = http.StatusText(serviceResp.Response().GetStatusCode())
		}
		if err := serviceResp.Error(); err != nil {
			return err
		}
		resp2Body, err = serviceResp.ExportResponse()
		if err != nil {
			return err
		}
		return nil
	})

	if err := wg.Wait(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	var ans profileResponse

	ans.Code = resp1Code
	ans.Code = max(resp1Code, resp2Code)

	if resp1Err != "" {
		ans.Error = resp1Err

	} else if resp2Err != "" {
		ans.Error = resp2Err

	} else if ok := json.Valid(resp1Body); !ok {
		ans.Error = "Unparseable JSON response for LemmaInfo"

	} else if ok := json.Valid(resp2Body); !ok {
		ans.Error = "Unparseable JSON response for PoSInfo"

	} else {
		ans.LemmaInfo = resp1Body
		ans.PoSInfo = resp2Body
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewGramatikatProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guardImpl.RequestInfo,
) (*GramatikatProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gramatikat proxy: %w", err)
	}
	return &GramatikatProxy{
		Proxy: proxy,
	}, nil
}
