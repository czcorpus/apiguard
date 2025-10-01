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

package cncauth

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/guard"
	guardImpl "github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/logging"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/session"
	"github.com/czcorpus/apiguard/telemetry"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Guard in the cncauth package allows access to any user
// with valid session ID - i.e. it does not matter whether
// the user is public (anonymous) or registered. But the guard
// is able to use the knowledge about user ID when determining
// how "gently" it should react to user's possible high request
// rate.
//
// One of its crucial functions is that it can allow access
// anonymous users to APIs which otherwise require registered
// users - not to prevent data access (it wouldn't make sense as
// APIGuard with this guard opens such data to anyone) but rather
// for situations where we (our APIs) highly prefer registered users
// so we can generate more concrete reports about usage of our services.
//
// A typical example:
// KonText API is only for registered users. And our WaG application
// (Slovo v kostce, Slovo v poezii) also uses KonText API but WaG
// is for all users (unregistered users even make up the majority
// of users there).
type Guard struct {
	location *time.Location

	tlmtrStorage telemetry.Storage

	// backendSessionCookie is a cookie used between APIGuard and API (e.g. KonText-API)
	// Please note that CNC central authentication cookie is typically the same
	// as backendSessionCookie in which sense the term "internal" may sound weird.
	// The reason is that APIGuard creates additional layer between user and CNC auth mechanism
	// in which case the former "external" becomes "internal" from APIGuard point of view.
	//
	// It typically looks like this:
	//   a) For direct user-service communication we have:
	//      [CNC user] ---- cookie1 ---> [CNC website (e.g. KonText, Treq)]
	//   b) For indirect user-API communication handled by APIGuard we have:
	//      [CNC user] ---- cookie2 --> [WaG] --- cookie2 ---> [APIGuard] --> cookie(2->1) --> [KonText API]
	// where cookie1 is internal, cookie2 is external, cookie(2->1) is cookie2 renamed to cookie1
	// But the user may also have both cookies:
	// [CNC user] ---- c1, c2 --> [WaG] --- c1, c2 ---> [APIGuard] --> cookie(2->1) --> [KonText API]
	// Here by default, APIGuard will ignore c1 (it again used c2 and renames it to c1 for KonText API)
	//
	// We also use the fact that in terms of value, both cookies are the same and in case we need
	// to check whether the frontend authentication (user logged via CNC login page)
	// provides a valid user, we refer to this backendSessionCookie - so don't be confused
	// by this.
	backendSessionCookie string

	// frontendSessionCookie is a cookie used between APIGuard client (e.g. WaG) and
	// APIGuard. This allows access for both authenticated CNC users (in which case
	// their credentials will be passed to a target API), and for public users in which
	// case APIGuard will use its capabilities (e.g. a configured fallback user account
	// created not for a human user but rather for an application) to authenticate
	// (possibly with some lowered permissions) such user to otherwise non-public CNC APIs.
	frontendSessionCookie string

	anonymousUsers common.AnonymousUsers

	rateLimiters map[string]*rate.Limiter

	confLimits []proxy.Limit

	rateLimitersMu sync.Mutex

	sessionValFactory func() session.HTTPSession

	userFinder guardImpl.UserFinder
}

func (kua *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return kua.anonymousUsers.IsAnonymous(userID)
}

// CalcDelay calculates a delay user deserves.
// CNC-auth guard applies only two delays:
// 1) zero for non-banned users
// 2) guard.UltraDuration which is basically a ban
func (kua *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error) {
	return 0, nil
}

func (kua *Guard) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	return kua.tlmtrStorage.LogAppliedDelay(respDelay, clientID)
}

// getFrontendOrBackendSession returns a user session value value along with the information
// whether the session was obtained via frontend cookie (see configuration
// for explanation).
// As the name suggests, the method first tries the frontend session and only if it gives
// no value, it looks also for backend session (which is typically cnc auth session).
func (kua *Guard) getFrontendOrBackendSession(
	req *http.Request,
) (cookieValue session.HTTPSession, isFrontend bool) {
	cookieValue = kua.sessionValFactory().UpdatedFrom(proxy.GetCookieValue(req, kua.frontendSessionCookie))
	if !cookieValue.IsZero() {
		isFrontend = true
		return
	}
	cookieValue = kua.sessionValFactory().UpdatedFrom(proxy.GetCookieValue(req, kua.backendSessionCookie))
	return
}

func (kua *Guard) getUserCNCSessionCookie(req *http.Request) *http.Cookie {
	cookie, err := req.Cookie(kua.backendSessionCookie)
	if err == http.ErrNoCookie {
		return nil
	}
	return cookie
}

func (kua *Guard) getUserCNCSessionID(req *http.Request) session.HTTPSession {
	v := proxy.GetCookieValue(req, kua.backendSessionCookie)
	return kua.sessionValFactory().UpdatedFrom(v)
}

// DetermineTrueUserID tests whether CNC authentication
// cookie (internal cookie in our terms) provides a valid
// non-anonymous user
func (kua *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {

	cookie := kua.getUserCNCSessionCookie(req)
	if cookie == nil {
		return common.InvalidUserID, nil
	}
	sessionVal := kua.getUserCNCSessionID(req)
	userID, err := kua.userFinder.FindUserBySession(sessionVal)
	if err != nil {
		return common.InvalidUserID, err
	}
	return userID, nil
}

func (kua *Guard) checkForBan(req *http.Request, clientID common.ClientID) (bool, error) {
	ip, _ := logging.ExtractRequestIdentifiers(req)
	isBanned, err := kua.tlmtrStorage.TestIPBan(net.ParseIP(ip))
	if err != nil {
		return isBanned, err
	}
	if isBanned {
		log.Debug().
			Str("guardType", "cncauth").
			Str("clientId", clientID.GetKey()).
			Msg("applied IP ban")
		return true, nil
	}
	return false, nil
}

// EvaluateRequest produces a HTTP response status
// proposal based on user activity.
// The function prefers frontend cookie. I.e. if a client (e.g. WaG)
// sends custom auth cookie, then the user identified by that cookie
// will be detected here - not the one indetified by CNC common autentication
// cookie.
func (analyzer *Guard) EvaluateRequest(req *http.Request, fallbackCookie *http.Cookie) guard.ReqEvaluation {
	var requiresFallbackCookie bool
	clientIP := logging.ExtractClientIP(req)

	cookieValue, _ := analyzer.getFrontendOrBackendSession(req)
	if cookieValue.IsZero() {
		proxy.LogCookies(req, log.Debug()).
			Str("backendCookie", analyzer.backendSessionCookie).
			Str("frontendCookie", analyzer.frontendSessionCookie).
			Msgf("failed to find authentication cookies")
		requiresFallbackCookie = true
		if fallbackCookie == nil { //note requiresFallbackCookie == true - it is because here there is no other way already
			return guard.ReqEvaluation{
				ProposedResponse:       http.StatusUnauthorized,
				ClientID:               common.InvalidUserID,
				Error:                  fmt.Errorf("session cookie not found"),
				RequiresFallbackCookie: requiresFallbackCookie,
			}

		} else {
			proxy.LogCookies(req, log.Debug()).Msg("using default cookie")
			cookieValue = session.CNCSessionValue{}.UpdatedFrom(fallbackCookie.Value)
		}
	}
	apiUserID, err := analyzer.userFinder.FindUserBySession(analyzer.sessionValFactory().UpdatedFrom(cookieValue.String()))
	if err != nil {
		return guard.ReqEvaluation{
			ProposedResponse:       http.StatusInternalServerError,
			ClientID:               apiUserID,
			RequiresFallbackCookie: requiresFallbackCookie,
			Error:                  fmt.Errorf("failed to determine userID: %w", err),
		}
	}
	if !apiUserID.IsValid() {
		if analyzer.userFinder.InvalidUserIsOK() {
			return guard.ReqEvaluation{
				ProposedResponse: http.StatusOK,
				ClientID:         common.InvalidUserID,
				SessionID:        "",
				Error:            nil,
			}
		}
		return guard.ReqEvaluation{
			ProposedResponse:       http.StatusUnauthorized,
			RequiresFallbackCookie: true,
		}
	}
	if len(analyzer.confLimits) > 0 {
		analyzer.rateLimitersMu.Lock()
		defer analyzer.rateLimitersMu.Unlock()
		limiter, exists := analyzer.rateLimiters[clientIP]
		if !exists {
			flimit := analyzer.confLimits[0]
			limiter = rate.NewLimiter(
				flimit.NormLimitPerSec(),
				flimit.BurstLimit,
			)
			analyzer.rateLimiters[clientIP] = limiter
		}
		if !limiter.Allow() {
			log.Debug().Str("clientIp", clientIP).Msg("limiting client with status 429")
			return guard.ReqEvaluation{
				ProposedResponse:       http.StatusTooManyRequests,
				ClientID:               apiUserID,
				SessionID:              cookieValue.String(),
				RequiresFallbackCookie: requiresFallbackCookie,
			}
		}
	}

	// test ip ban
	banned, err := analyzer.checkForBan(req, common.ClientID{IP: clientIP, ID: apiUserID})
	if err != nil {
		return guard.ReqEvaluation{
			ProposedResponse:       http.StatusInternalServerError,
			RequiresFallbackCookie: requiresFallbackCookie,
			Error:                  err,
		}
	}
	if banned {
		return guard.ReqEvaluation{
			ProposedResponse:       http.StatusForbidden,
			RequiresFallbackCookie: requiresFallbackCookie,
		}
	}
	return guard.ReqEvaluation{
		ProposedResponse:       http.StatusOK,
		ClientID:               apiUserID,
		SessionID:              cookieValue.String(),
		RequiresFallbackCookie: requiresFallbackCookie,
		Error:                  err,
	}
}

func New(
	globalCtx *globctx.Context,
	backendSessionCookie string,
	frontendSessionCookie string,
	sessionType session.SessionType,
	confLimits []proxy.Limit,

) *Guard {
	return &Guard{
		tlmtrStorage:          globalCtx.TelemetryDB,
		location:              globalCtx.TimezoneLocation,
		backendSessionCookie:  backendSessionCookie,
		frontendSessionCookie: frontendSessionCookie,
		anonymousUsers:        globalCtx.AnonymousUserIDs,
		confLimits:            confLimits,
		rateLimiters:          make(map[string]*rate.Limiter),
		sessionValFactory:     guardImpl.CreateSessionValFactory(sessionType),
		userFinder:            guardImpl.NewUserFinder(globalCtx),
	}
}
