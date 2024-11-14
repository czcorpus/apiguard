// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package sessionmap

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/services/logging"
	"apiguard/session"
	"apiguard/telemetry"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Guard in the sessionmap package allows access to any user
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
	db *sql.DB

	location *time.Location

	tlmtrStorage telemetry.Storage

	// internalSessionCookie is a cookie used between APIGuard and API (e.g. KonText-API)
	// Please note that CNC central authentication cookie is typically the same
	// as internalSessionCookie in which sense the term "internal" may sound weird.
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
	// to check whether an external authentication (user logged via CNC login page)
	// provides valid user, we refer to this internalSessionCookie - so don't be confused
	// by this.
	internalSessionCookie string

	// externalSessionCookie is a cookie used between APIGuard client (e.g. WaG) and
	// APIGuard. This allows access for both authenticated CNC users (in which case
	// their credentials will be passed to a target API), and for public users in which
	// case APIGuard will use its capabilities (e.g. a configured fallback user account
	// created not for a human user but rather for an application) to authenticate
	// (possibly with some lowered permissions) such user to otherwise non-public CNC APIs.
	externalSessionCookie string

	anonymousUsers common.AnonymousUsers

	rateLimiters map[string]*rate.Limiter

	confLimits []proxy.Limit

	rateLimitersMu sync.Mutex

	sessionValFactory func() session.HTTPSession
}

func (kua *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return kua.anonymousUsers.IsAnonymous(userID)
}

// CalcDelay calculates a delay user deserves.
// SessionMappingGuard applies only two delays:
// 1) zero for non-banned users
// 2) guard.UltraDuration which is basically a ban
func (kua *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error) {
	return 0, nil
}

func (kua *Guard) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	return kua.tlmtrStorage.LogAppliedDelay(respDelay, clientID)
}

// getSession returns a user session value value along with the information
// whether the session was obtained via external cookie (see configuration
// for explanation).
func (kua *Guard) getSession(req *http.Request) (cookieValue string, isExternal bool) {
	cookieValue = proxy.GetCookieValue(req, kua.externalSessionCookie)
	if cookieValue != "" {
		isExternal = true
		return
	}
	cookieValue = proxy.GetCookieValue(req, kua.internalSessionCookie)
	return
}

// GetSessionID extracts relevant user session ID from the provided Request.
func (kua *Guard) GetSessionID(req *http.Request) string {
	v, _ := kua.getSession(req)
	if v != "" {
		return strings.SplitN(v, "-", 2)[0]
	}
	return ""
}

func (kua *Guard) getUserCNCSessionCookie(req *http.Request) *http.Cookie {
	cookie, err := req.Cookie(kua.internalSessionCookie)
	if err == http.ErrNoCookie {
		return nil
	}
	return cookie
}

func (kua *Guard) getUserCNCSessionID(req *http.Request) session.HTTPSession {
	v := proxy.GetCookieValue(req, kua.internalSessionCookie)
	return kua.sessionValFactory().UpdatedFrom(v)
}

// DetermineTrueUserID tests whether CNC authentication
// cookie (internal cookie in our terms) provides a valid
// non-anonymous user
func (kua *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {

	cookie := kua.getUserCNCSessionCookie(req)
	if kua.db == nil || cookie == nil {
		return common.InvalidUserID, nil
	}
	sessionVal := kua.getUserCNCSessionID(req)
	userID, err := guard.FindUserBySession(kua.db, sessionVal)
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
			Str("guardType", "sessionmap").
			Str("clientId", clientID.GetKey()).
			Msg("applied IP ban")
		return true, nil
	}
	return false, nil
}

// EvaluateRequest produces a HTTP response status
// proposal based on user activity.
// The function prefers external cookie. I.e. if a client (e.g. WaG)
// sends custom auth cookie, then the user identified by that cookie
// will be detected here - not the one indetified by CNC common autentication
// cookie.
func (analyzer *Guard) EvaluateRequest(req *http.Request) guard.ReqEvaluation {

	clientIP, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusUnauthorized,
			ClientID:         common.InvalidUserID,
			SessionID:        "",
			Error:            fmt.Errorf("failed to determine user IP: %w", err),
		}
	}

	if analyzer.db == nil {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusOK,
			ClientID:         common.InvalidUserID,
			SessionID:        "",
			Error:            nil,
		}
	}
	cookieValue, _ := analyzer.getSession(req)
	userID := common.InvalidUserID
	if cookieValue == "" {
		proxy.LogCookies(req, log.Debug()).
			Str("internalCookie", analyzer.internalSessionCookie).
			Str("externalCookie", analyzer.externalSessionCookie).
			Msgf("failed to find authentication cookies")
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusUnauthorized,
			ClientID:         common.InvalidUserID,
			Error:            fmt.Errorf("session cookie not found"),
		}

	} else {
		var err error
		userID, err = analyzer.DetermineTrueUserID(req)
		if err != nil {
			return guard.ReqEvaluation{
				ProposedResponse: http.StatusInternalServerError,
				ClientID:         common.InvalidUserID,
				Error:            fmt.Errorf("failed to determine userID: %w", err),
			}
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
			return guard.ReqEvaluation{
				ProposedResponse: http.StatusTooManyRequests,
				ClientID:         userID,
				SessionID:        cookieValue,
			}
		}
	}

	// test ip ban
	banned, err := analyzer.checkForBan(req, common.ClientID{IP: clientIP, ID: userID})
	if err != nil {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusInternalServerError,
			Error:            err,
		}
	}
	if banned {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusForbidden,
		}
	}
	return guard.ReqEvaluation{
		ProposedResponse: http.StatusOK,
		ClientID:         userID,
		SessionID:        cookieValue,
		Error:            err,
	}
}

func New(
	globalCtx *globctx.Context,
	internalSessionCookie string,
	externalSessionCookie string,
	sessionType session.SessionType,
	confLimits []proxy.Limit,

) *Guard {
	return &Guard{
		db:                    globalCtx.CNCDB,
		tlmtrStorage:          globalCtx.TelemetryDB,
		location:              globalCtx.TimezoneLocation,
		internalSessionCookie: internalSessionCookie,
		externalSessionCookie: externalSessionCookie,
		anonymousUsers:        globalCtx.AnonymousUserIDs,
		confLimits:            confLimits,
		rateLimiters:          make(map[string]*rate.Limiter),
		sessionValFactory:     guard.CreateSessionValFactory(sessionType),
	}
}
