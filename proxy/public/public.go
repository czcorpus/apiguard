// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package public

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/session"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	DfltAuthCookieName     = "cnc_toolbar_sid"
	DfltUserIDHeaderName   = "X-Api-User"
	DfltTrueUserHeaderName = "X-True-User"
	DfltReadTimeoutSecs    = 30
)

var (
	DfltBackendURL  = mustParseURL("http://127.0.0.1:8080/")
	DflFfrontendURL = mustParseURL("http://127.0.0.1/")
)

type PublicAPIProxyOpts struct {

	// ServicePath is a service path as understood by internal router
	// e.g. /service/3/gunstick
	ServicePath string

	// ServiceKey is a unique service id - e.g. 3/gunstick
	ServiceKey string

	BackendURL          *url.URL
	FrontendURL         *url.URL
	AuthCookieName      string
	UserIDHeaderName    string
	ReadTimeoutSecs     int
	ResponseInterceptor func(*proxy.ProxiedResponse)
}

// APIProxy is a service proxy which - in general - does not
// forbid any user from accessing protected API. But it still
// distinguishes between logged-in users and anonymous ones. And
// it may throttle requests with some favouring of logged-in users.
type APIProxy struct {
	servicePath         string
	serviceKey          string
	BackendURL          *url.URL
	FrontendURL         *url.URL
	authCookieName      string
	userIDHeaderName    string
	readTimeoutSecs     int
	client              *http.Client
	cache               proxy.Cache
	basicProxy          *proxy.APIProxy
	clientCounter       chan<- common.ClientID
	guard               guard.ServiceGuard
	db                  *sql.DB
	tzLocation          *time.Location
	responseInterceptor func(resp *proxy.ProxiedResponse)
	tDBWriter           reporting.ReportingWriter
}

func mustParseURL(rawUrl string) *url.URL {
	u, err := url.Parse(rawUrl)
	if err != nil {
		panic(err)
	}
	return u
}

func (prox *APIProxy) getUserCNCSessionCookie(req *http.Request) *http.Cookie {
	cookie, err := req.Cookie(prox.authCookieName)
	if err == http.ErrNoCookie {
		return nil
	}
	return cookie
}

func (prox *APIProxy) getUserCNCSessionID(req *http.Request) session.HTTPSession {
	v := proxy.GetCookieValue(req, prox.authCookieName)
	return session.CNCSessionValue{}.UpdatedFrom(v)
}

func (prox *APIProxy) RestrictResponseTime(ctx *gin.Context, clientID common.ClientID) error {
	respDelay, err := prox.guard.CalcDelay(ctx.Request, clientID)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		log.Error().Err(err).Msg("failed to analyze client")
		return err
	}
	log.Debug().Msgf("Client is going to wait for %v", respDelay)
	if respDelay.Seconds() >= float64(prox.readTimeoutSecs) {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("service overloaded"),
			http.StatusServiceUnavailable,
		)
		return err
	}
	time.Sleep(respDelay)
	return nil
}

func (prox *APIProxy) determineTrueUserID(
	req *http.Request,
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

func (prox *APIProxy) AnyPath(ctx *gin.Context) {
	var humanID common.UserID
	path := ctx.Request.URL.Path
	rt0 := time.Now().In(prox.tzLocation)

	defer func(userID *common.UserID) {
		log.Debug().
			Str("backendURL", prox.BackendURL.String()).
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

	var err error
	if prox.userIDHeaderName != "" {
		humanID, err = prox.determineTrueUserID(ctx.Request)
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
	}

	clientID := common.ClientID{
		IP: ctx.RemoteIP(),
		ID: humanID,
	}
	prox.clientCounter <- clientID

	reqProps := prox.guard.EvaluateRequest(ctx.Request)
	if reqProps.Error != nil {
		log.Error().Err(reqProps.Error).Msgf("failed to proxy request")
		proxy.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %s", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ForbidsAccess() {
		http.Error(ctx.Writer, http.StatusText(reqProps.ProposedResponse), reqProps.ProposedResponse)
		return
	}

	err = prox.RestrictResponseTime(ctx, clientID)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
	}
	if prox.userIDHeaderName != "" && humanID.IsValid() {
		ctx.Request.Header.Set(prox.userIDHeaderName, humanID.String())
	}

	resp, err := prox.cache.Get(ctx.Request)
	if err != nil {
		if err != proxy.ErrCacheMiss {
			log.Error().
				Err(err).
				Str("serviceKey", prox.serviceKey).
				Msg("unexpected caching error in PublicAPIProxy - this should be resolved")
		}
		resp = prox.basicProxy.Request(
			// TODO use some path builder here
			internalPath,
			ctx.Request.URL.Query(),
			ctx.Request.Method,
			ctx.Request.Header,
			ctx.Request.Body,
		)
		if resp.GetError() == nil {
			if err := prox.cache.Set(ctx.Request, resp); err != nil {
				log.Error().
					Err(err).
					Str("serviceKey", prox.serviceKey).
					Msg("PublicAPIProxy failed to cache backend response")
			}
		}
		logging.AddLogEvent(ctx, "isCached", false)

	} else {
		logging.AddLogEvent(ctx, "isCached", true)
	}
	prox.tDBWriter.Write(&reporting.ProxyProcReport{
		DateTime: time.Now().In(prox.tzLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   resp.GetStatusCode(),
		Service:  prox.serviceKey,
		IsCached: resp.IsCached(),
	})
	pr, ok := resp.(*proxy.ProxiedResponse)
	if ok {
		prox.responseInterceptor(pr)

	} else {
		log.Debug().
			Str("req", ctx.Request.URL.String()).
			Msg("cannot apply public proxy interceptor - not *ProxiedResponse type")
	}
	for k, v := range resp.GetHeaders() {
		ctx.Writer.Header().Set(k, v[0])
	}
	ctx.Writer.WriteHeader(resp.GetStatusCode())
	ctx.Writer.Write(resp.GetBody())
}

// NewAPIProxy
// note: all the options in `opts` are indeed optional but most of the time,
// reasonable custom values are preferred. Also, any non-filled option is
// logged as a warning providing also a respective fallback value.
func NewAPIProxy(
	globalCtx *globctx.Context,
	basicProxy *proxy.APIProxy,
	sid int,
	client *http.Client,
	clientCounter chan<- common.ClientID,
	guard guard.ServiceGuard,
	opts PublicAPIProxyOpts,

) *APIProxy {

	respInt := opts.ResponseInterceptor
	if respInt == nil {
		respInt = func(pr *proxy.ProxiedResponse) {}
	}

	p := &APIProxy{
		client:              client,
		basicProxy:          basicProxy,
		clientCounter:       clientCounter,
		cache:               globalCtx.Cache,
		guard:               guard,
		db:                  globalCtx.CNCDB,
		responseInterceptor: respInt,
		tDBWriter:           globalCtx.ReportingWriter,
		tzLocation:          globalCtx.TimezoneLocation,
	}

	if opts.AuthCookieName == "" {
		p.authCookieName = DfltAuthCookieName
		log.Warn().Str("value", DfltAuthCookieName).Msg("AuthCookieName not set for public proxy, using default")

	} else {
		p.authCookieName = opts.AuthCookieName
	}

	if opts.FrontendURL == nil {
		p.FrontendURL = DflFfrontendURL
		log.Warn().Str("value", DflFfrontendURL.String()).Msg("frontendUrl not set for public proxy, using default")

	} else {
		p.FrontendURL = opts.FrontendURL
	}

	if opts.BackendURL == nil {
		p.BackendURL = DfltBackendURL
		log.Warn().Str("value", DfltBackendURL.String()).Msg("backendUrl not set for public proxy, using default")

	} else {
		p.BackendURL = opts.BackendURL
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

	p.serviceKey = opts.ServiceKey
	p.servicePath = fmt.Sprintf("/service/%s", p.serviceKey)

	if opts.UserIDHeaderName == "" {
		log.Warn().Msg("UserIDHeaderName not set for public proxy, no CNC user ID will be passed via headers")

	} else {
		p.userIDHeaderName = opts.UserIDHeaderName
	}

	return p
}
