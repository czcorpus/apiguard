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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	DfltServiceName      = "public_proxy_service"
	DfltAuthCookieName   = "cnc_toolbar_sid"
	DfltUserIDHeaderName = "X-Api-User"
	DfltReadTimeoutSecs  = 30
)

var (
	DfltInternalURL = mustParseURL("http://127.0.0.1:8080/")
	DfltExternalURL = mustParseURL("http://127.0.0.1/")
)

type PublicAPIProxyOpts struct {
	ServiceName      string
	InternalURL      *url.URL
	ExternalURL      *url.URL
	AuthCookieName   string
	UserIDHeaderName string
	ReadTimeoutSecs  int
}

type PublicAPIProxy struct {
	serviceName      string
	servicePath      string
	InternalURL      *url.URL
	ExternalURL      *url.URL
	authCookieName   string
	userIDHeaderName string
	readTimeoutSecs  int
	client           *http.Client
	basicProxy       *APIProxy
	ipCounter        chan<- string
	reqAnalyzer      guard.ReqAnalyzer
	db               *sql.DB
}

func mustParseURL(rawUrl string) *url.URL {
	u, err := url.Parse(rawUrl)
	if err != nil {
		panic(err)
	}
	return u
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
	var humanID common.UserID
	path := ctx.Request.URL.Path

	defer func(userID *common.UserID) {
		log.Debug().
			Str("internalURL", prox.InternalURL.String()).
			Str("requestPath", path).
			Str("servicePath", prox.servicePath).
			Int("userID", int(humanID)).
			Msg("asked to process proxy paths (deferred message)")
	}(&humanID)

	if !strings.HasPrefix(path, prox.servicePath) {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("unknown service path (expected %s)", prox.servicePath), http.StatusNotFound)
		return
	}
	internalPath := strings.TrimPrefix(path, prox.servicePath)

	prox.ipCounter <- ctx.RemoteIP()

	var err error
	if prox.userIDHeaderName != "" {
		humanID, err = prox.userInternalCookieStatus(ctx.Request, prox.serviceName)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
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
	if prox.userIDHeaderName != "" {
		ctx.Writer.Header().Set(prox.userIDHeaderName, humanID.String())
	}
	ctx.Writer.WriteHeader(resp.GetStatusCode())
	ctx.Writer.Write(resp.GetBody())
}

// NewPublicAPIProxy
// note: all the options in `opts` are indeed optional but most of the time,
// reasonable custom values are preferred. Also, any non-filled option is
// logged as a warning providing also a respective fallback value.
func NewPublicAPIProxy(
	basicProxy *APIProxy,
	client *http.Client,
	ipCounter chan<- string,
	reqAnalyzer guard.ReqAnalyzer,
	db *sql.DB,
	opts PublicAPIProxyOpts,

) *PublicAPIProxy {

	p := &PublicAPIProxy{

		client:      client,
		basicProxy:  basicProxy,
		ipCounter:   ipCounter,
		reqAnalyzer: reqAnalyzer,
		db:          db,
	}

	if opts.AuthCookieName == "" {
		p.authCookieName = DfltAuthCookieName
		log.Warn().Str("value", DfltAuthCookieName).Msg("AuthCookieName not set for public proxy, using default")

	} else {
		p.authCookieName = opts.AuthCookieName
	}

	if opts.ExternalURL == nil {
		p.ExternalURL = DfltExternalURL
		log.Warn().Str("value", DfltExternalURL.String()).Msg("ExternalURL not set for public proxy, using default")

	} else {
		p.ExternalURL = opts.ExternalURL
	}

	if opts.InternalURL == nil {
		p.InternalURL = DfltInternalURL
		log.Warn().Str("value", DfltInternalURL.String()).Msg("InternalURL not set for public proxy, using default")

	} else {
		p.InternalURL = opts.InternalURL
	}

	if opts.ReadTimeoutSecs == 0 {
		p.readTimeoutSecs = DfltReadTimeoutSecs
		log.Warn().Int("value", DfltReadTimeoutSecs).Msg("ReadTimeoutSecs not set for public proxy, using default")

	} else {
		p.readTimeoutSecs = opts.ReadTimeoutSecs
	}

	if opts.AuthCookieName == "" {
		p.authCookieName = DfltAuthCookieName
		log.Warn().Str("value", DfltAuthCookieName).Msg("AuthCookieName not set for public proxy, using default")

	} else {
		p.authCookieName = opts.AuthCookieName
	}

	if opts.ServiceName == "" {
		p.serviceName = DfltServiceName
		log.Warn().Str("value", DfltServiceName).Msg("ServiceName not set for public proxy, using default")

	} else {
		p.serviceName = opts.ServiceName
	}
	p.servicePath = fmt.Sprintf("/service/%s", p.serviceName)

	if opts.UserIDHeaderName == "" {
		log.Warn().Msg("UserIDHeaderName not set for public proxy, no CNC user ID will be passed via headers")

	} else {
		p.userIDHeaderName = opts.UserIDHeaderName
	}

	return p
}
