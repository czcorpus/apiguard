// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package userdb

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/services/logging"
	"apiguard/session"
	"apiguard/users"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CNCUserAnalyzer provides access to user request and is able
// to access CNC session database to evaluate user permissions.
// Because of possible cookie mapping for some of services, the
// analyzer may look into more than one cookie. But it is up
// to a consumer to configure proper order of cookie lookup
// (SessionCookieNames)
type CNCUserAnalyzer struct {
	db             *sql.DB
	delayStats     *guard.DelayStats
	location       *time.Location
	userTableProps users.UserTableProps

	// internalSessionCookie is a cookie used between APIGuard and API (e.g. KonText-API)
	// Please note that CNC central authentication cookie is typically the same
	// as internalSessionCookie in which sense the term "internal" may sound weird.
	// It typically looks like this:
	// [CNC user] ---- cookie1 ---> [CNC website (e.g. KonText, Treq)]
	// [CNC user] ---- cookie2 --> [WaG] --- cookie2 ---> [APIGuard] --> cookie(2->1) --> [KonText API]
	// where cookie1 is internal, cookie2 is external, cookie(2->1) is cookie2 renamed to cookie1
	// But the user may also have both cookies:
	// [CNC user] ---- c1, c2 --> [WaG] --- c1, c2 ---> [APIGuard] --> cookie(2->1) --> [KonText API]
	// Here by default, APIGuard will ignore c1 (it again used c2 and renames it to c1 for KonText API)
	//
	// We also use the fact that the both cookies are the same and in case we need
	// to check whether an external authentication (user logged via CNC login page)
	// provides valid user, we refer to this internalSessionCookie - so don't be confused
	// by this.
	internalSessionCookie string

	// externalSessionCookie is a cookie used between APIGuard client (e.g. WaG) and
	// APIGuard
	externalSessionCookie string
	AnonymousUserID       common.UserID
}

// CalcDelay calculates a delay user deserves.
// CNCUserAnalyzer applies only two delays:
// 1) zero for non-banned users
// 2) guard.UltraDuration which is basically a ban
func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (guard.DelayInfo, error) {
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

func (kua *CNCUserAnalyzer) LogAppliedDelay(respDelay guard.DelayInfo, clientIP string) error {
	err := kua.delayStats.LogAppliedDelay(respDelay, clientIP)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register delay log")
	}
	return err
}

func (kua *CNCUserAnalyzer) getSessionValue(req *http.Request) string {
	cookieValue := proxy.GetCookieValue(req, kua.externalSessionCookie)
	if cookieValue != "" {
		return cookieValue
	}
	cookieValue = proxy.GetCookieValue(req, kua.internalSessionCookie)
	return cookieValue
}

// GetSessionID extracts relevant user session ID from the provided Request.
func (kua *CNCUserAnalyzer) GetSessionID(req *http.Request) string {
	v := kua.getSessionValue(req)
	if v != "" {
		return strings.SplitN(v, "-", 2)[0]
	}
	return ""
}

func (kua *CNCUserAnalyzer) getUserCNCSessionCookie(req *http.Request) *http.Cookie {
	cookie, err := req.Cookie(kua.internalSessionCookie)
	if err == http.ErrNoCookie {
		return nil
	}
	return cookie
}

func (kua *CNCUserAnalyzer) getUserCNCSessionID(req *http.Request) session.CNCSessionValue {
	v := proxy.GetCookieValue(req, kua.internalSessionCookie)
	ans := session.CNCSessionValue{}
	ans.UpdateFrom(v)
	return ans
}

// UserInternalCookieStatus tests whether CNC authentication
// cookie (internal cookie in our terms) provides a valid
// non-anonymous user
func (kua *CNCUserAnalyzer) UserInternalCookieStatus(
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

// UserInducedResponseStatus produces a HTTP response status
// proposal based on user activity.
// The function prefers external cookie. I.e. if a client (e.g. WaG)
// sends custom auth cookie, then the user identified by that cookie
// will be detected here - not the one indetified by CNC common autentication
// cookie.
func (analyzer *CNCUserAnalyzer) UserInducedResponseStatus(
	req *http.Request,
	serviceName string,
) guard.ReqProperties {

	if analyzer.db == nil {
		return guard.ReqProperties{
			ProposedStatus: http.StatusOK,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          nil,
		}
	}
	cookieValue := analyzer.getSessionValue(req)
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
	sessionID := analyzer.GetSessionID(req)
	banned, userID, err := guard.FindBanBySession(analyzer.db, analyzer.location, sessionID, serviceName)
	if err == sql.ErrNoRows || userID == analyzer.AnonymousUserID || !userID.IsValid() {
		log.Debug().Msgf("session %s not present in database", sessionID)
		return guard.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          nil,
		}
	}
	status := http.StatusOK
	if banned {
		status = http.StatusForbidden
	}
	return guard.ReqProperties{
		ProposedStatus: status,
		UserID:         userID,
		SessionID:      sessionID,
		Error:          err,
	}
}

func NewCNCUserAnalyzer(
	db *sql.DB,
	delayStats *guard.DelayStats,
	location *time.Location,
	userTableProps users.UserTableProps,
	internalSessionCookie string,
	externalSessionCookie string,
	anonymousUserID common.UserID,

) *CNCUserAnalyzer {
	return &CNCUserAnalyzer{
		db:                    db,
		delayStats:            delayStats,
		location:              location,
		userTableProps:        userTableProps,
		internalSessionCookie: internalSessionCookie,
		externalSessionCookie: externalSessionCookie,
		AnonymousUserID:       anonymousUserID,
	}
}
