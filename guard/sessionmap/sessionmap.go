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

// Guard analyzes user request and based on session,
// detected user ID and IP address it can calculate suitable delay
// and response status.
//
// Its key distinct property is that it can
// handle configurations with user session remapping where some
// outer session cookie (typically - cnc_toolbar_sid) which is used
// for communication between a user and APIGuard is internally remapped
// (during proxying) to a defined internal cookie which is used
// in communication between APIGuard and protected application server.
// This is used e.g. with WaG.
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

	AnonymousUserIDs common.AnonymousUsers

	rateLimiters map[string]*rate.Limiter

	rateLimitersMu sync.Mutex
}

// CalcDelay calculates a delay user deserves.
// SessionMappingGuard applies only two delays:
// 1) zero for non-banned users
// 2) guard.UltraDuration which is basically a ban
func (kua *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (guard.DelayInfo, error) {
	ip, _ := logging.ExtractRequestIdentifiers(req)
	delayInfo := guard.DelayInfo{
		Delay: time.Duration(0),
		IsBan: false,
	}
	isBanned, err := guard.TestIPBan(kua.db, net.ParseIP(ip), kua.location)
	if err != nil {
		return delayInfo, err
	}
	if isBanned {
		delayInfo.Delay = guard.UltraDuration
		delayInfo.IsBan = true
		return delayInfo, nil
	}
	return delayInfo, nil
}

func (kua *Guard) LogAppliedDelay(respDelay guard.DelayInfo, clientID common.ClientID) error {
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

func (kua *Guard) getUserCNCSessionID(req *http.Request) session.CNCSessionValue {
	v := proxy.GetCookieValue(req, kua.internalSessionCookie)
	ans := session.CNCSessionValue{}
	ans.UpdateFrom(v)
	return ans
}

// UserInternalCookieStatus tests whether CNC authentication
// cookie (internal cookie in our terms) provides a valid
// non-anonymous user
func (kua *Guard) UserInternalCookieStatus(
	req *http.Request,
	serviceName string,
) (common.UserID, error) {

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

// ClientInducedRespStatus produces a HTTP response status
// proposal based on user activity.
// The function prefers external cookie. I.e. if a client (e.g. WaG)
// sends custom auth cookie, then the user identified by that cookie
// will be detected here - not the one indetified by CNC common autentication
// cookie.
func (analyzer *Guard) ClientInducedRespStatus(
	req *http.Request,
	serviceName string,
) guard.ReqProperties {

	clientIP, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		return guard.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          fmt.Errorf("failed to determine user IP: %w", err),
		}
	}

	if analyzer.db == nil {
		return guard.ReqProperties{
			ProposedStatus: http.StatusOK,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          nil,
		}
	}
	cookieValue, _ := analyzer.getSession(req)
	if cookieValue == "" {
		proxy.LogCookies(req, log.Debug()).
			Str("internalCookie", analyzer.internalSessionCookie).
			Str("externalCookie", analyzer.externalSessionCookie).
			Msgf("failed to find authentication cookies")
		return guard.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          fmt.Errorf("session cookie not found"),
		}
	}

	status := http.StatusOK
	analyzer.rateLimitersMu.Lock()
	defer analyzer.rateLimitersMu.Unlock()
	limiter, exists := analyzer.rateLimiters[clientIP]
	if !exists {
		analyzer.rateLimiters[clientIP] = rate.NewLimiter()
	}

	return guard.ReqProperties{
		ProposedStatus: status,
		UserID:         userID,
		SessionID:      sessionID,
		Error:          err,
	}
}

func New(
	globalCtx *globctx.Context,
	internalSessionCookie string,
	externalSessionCookie string,
	anonymousUserIDs common.AnonymousUsers,

) *Guard {
	return &Guard{
		db:                    globalCtx.CNCDB,
		location:              globalCtx.TimezoneLocation,
		internalSessionCookie: internalSessionCookie,
		externalSessionCookie: externalSessionCookie,
		AnonymousUserIDs:      anonymousUserIDs,
		rateLimiters:          make(map[string]*rate.Limiter),
	}
}
