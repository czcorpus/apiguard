// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/session"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type PublicAPIProxy struct {
	serviceName      string
	InternalURL      *url.URL
	ExternalURL      *url.URL
	authCookieName   string
	userIDHeaderName string
	client           *http.Client
	basicProxy       *APIProxy
	readTimeoutSecs  int
	ipCounter        chan<- string
	reqAnalyzer      guard.ReqAnalyzer
	db               *sql.DB
}

func (prox *PublicAPIProxy) getUserCNCSessionCookie(req *http.Request) *http.Cookie {
	cookie, err := req.Cookie(prox.authCookieName)
	if err == http.ErrNoCookie {
		return nil
	}
	return cookie
}

func (prox *PublicAPIProxy) getUserCNCSessionID(req *http.Request) session.CNCSessionValue {
	v := GetCookieValue(req, prox.authCookieName)
	ans := session.CNCSessionValue{}
	ans.UpdateFrom(v)
	return ans
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

func (prox *PublicAPIProxy) userInternalCookieStatus(
	req *http.Request,
	serviceName string,
) (common.UserID, error) {

	cookie := prox.getUserCNCSessionCookie(req)
	if prox.db == nil || cookie == nil {
		return common.InvalidUserID, nil
	}
	sessionVal := prox.getUserCNCSessionID(req)
	userID, err := guard.FindUserBySession(prox.db, sessionVal)
	if err != nil {
		return common.InvalidUserID, err
	}
	return userID, nil
}

func (prox *PublicAPIProxy) AnyPath(ctx *gin.Context) {
	path := ctx.Request.URL.Path
	var internalPath string
	if strings.HasPrefix(path, prox.ExternalURL.Path) {
		internalPath = strings.TrimLeft(path, prox.ExternalURL.Path)
	}

	prox.ipCounter <- ctx.RemoteIP()

	humanID, err := prox.userInternalCookieStatus(ctx.Request, prox.serviceName)
	if err != nil {
		log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
	}

	err = prox.RestrictResponseTime(ctx)
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
	ctx.Writer.Header().Set(prox.userIDHeaderName, humanID.String())
	ctx.Writer.WriteHeader(resp.GetStatusCode())
	ctx.Writer.Write(resp.GetBody())
}

// NewPublicAPIProxy
// TODO: refactor - there is too much arguments in the function
func NewPublicAPIProxy(
	serviceName string,
	internalURL *url.URL,
	externalURL *url.URL,
	authCookieName string,
	userIDHeaderName string,
	readTimeoutSecs int,
	basicProxy *APIProxy,
	client *http.Client,
	ipCounter chan<- string,
	reqAnalyzer guard.ReqAnalyzer,
	db *sql.DB,

) *PublicAPIProxy {

	return &PublicAPIProxy{
		serviceName:      serviceName,
		InternalURL:      internalURL,
		ExternalURL:      externalURL,
		authCookieName:   authCookieName,
		userIDHeaderName: userIDHeaderName,
		readTimeoutSecs:  readTimeoutSecs,
		client:           client,
		basicProxy:       basicProxy,
		ipCounter:        ipCounter,
		reqAnalyzer:      reqAnalyzer,
		db:               db,
	}
}
