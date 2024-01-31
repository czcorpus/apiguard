// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"apiguard/guard"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type PublicAPIProxy struct {
	InternalURL     *url.URL
	ExternalURL     *url.URL
	client          *http.Client
	basicProxy      *APIProxy
	readTimeoutSecs int
	ipCounter       chan<- string
	reqAnalyzer     guard.ReqAnalyzer
}

func (prox *PublicAPIProxy) RestrictResponseTime(ctx *gin.Context) error {
	respDelay, err := prox.reqAnalyzer.CalcDelay(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		log.Error().Err(err).Msg("failed to analyze client")
		return err
	}
	log.Debug().Msgf("Client is going to wait for %v", respDelay.Delay)
	if respDelay.Delay.Seconds() >= float64(prox.readTimeoutSecs) {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("service overloaded"),
			http.StatusServiceUnavailable,
		)
		return err
	}
	time.Sleep(respDelay.Delay)
	return nil
}

func (prox *PublicAPIProxy) AnyPath(ctx *gin.Context) {
	path := ctx.Request.URL.Path
	var internalPath string
	if strings.HasPrefix(path, prox.ExternalURL.Path) {
		internalPath = strings.TrimLeft(path, prox.ExternalURL.Path)
	}

	prox.ipCounter <- ctx.RemoteIP()

	err := prox.RestrictResponseTime(ctx)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
	}

	resp := prox.basicProxy.Request(
		// TODO use some path builder here
		internalPath,
		ctx.Request.URL.Query(),
		ctx.Request.Method,
		ctx.Request.Header,
		ctx.Request.Body,
	)

	for k, v := range resp.GetHeaders() {
		ctx.Writer.Header().Set(k, v[0])
	}
	ctx.Writer.WriteHeader(resp.GetStatusCode())
	ctx.Writer.Write(resp.GetBody())
}

func NewPublicAPIProxy(
	internalURL *url.URL,
	externalURL *url.URL,
	readTimeoutSecs int,
	basicProxy *APIProxy,
	client *http.Client,
	ipCounter chan<- string,
	reqAnalyzer guard.ReqAnalyzer,

) *PublicAPIProxy {

	return &PublicAPIProxy{
		InternalURL:     internalURL,
		ExternalURL:     externalURL,
		readTimeoutSecs: readTimeoutSecs,
		client:          client,
		basicProxy:      basicProxy,
		ipCounter:       ipCounter,
		reqAnalyzer:     reqAnalyzer,
	}
}
