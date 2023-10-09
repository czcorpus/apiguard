// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package analyzer

import (
	"apiguard/botwatch"
	"apiguard/cncdb"
	"apiguard/common"
	"apiguard/services"
	"apiguard/services/logging"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CNCUserAnalyzer provides access to user request and is able
// too access CNC session database to evaluate user permissions.
// Because of possible cookie mapping for some of services, the
// analyzer may look into more than one cookie. But it is up
// to a consumer to configure proper order of cookie lookup
// (SessionCookieNames)
type CNCUserAnalyzer struct {
	db             *sql.DB
	location       *time.Location
	userTableProps cncdb.UserTableProps

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

// CalcDelay calculates a delay user deserves. CNCUserAnalyzer
// returns 0.
func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	ip, _ := logging.ExtractRequestIdentifiers(req)
	isBanned, err := cncdb.TestIPBan(kua.db, net.ParseIP(ip), kua.location)
	if err != nil {
		return 0, err
	}
	if isBanned {
		return botwatch.UltraDuration, nil
	}
	return 0, nil
}

func (kua *CNCUserAnalyzer) LogAppliedDelay(respDelay time.Duration) error {
	return nil // TODO
}

func (kua *CNCUserAnalyzer) getSessionValue(req *http.Request) string {
	cookieValue := services.GetCookieValue(req, kua.externalSessionCookie)
	if cookieValue != "" {
		return cookieValue
	}
	cookieValue = services.GetCookieValue(req, kua.internalSessionCookie)
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

func (kua *CNCUserAnalyzer) getUserCNCSessionID(req *http.Request) string {
	v := services.GetCookieValue(req, kua.internalSessionCookie)
	if v != "" {
		return strings.SplitN(v, "-", 2)[0]
	}
	return v
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
	internalSessionID := kua.getUserCNCSessionID(req)
	userID, err := cncdb.FindUserBySession(kua.db, internalSessionID)
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
) services.ReqProperties {

	if analyzer.db == nil {
		return services.ReqProperties{
			ProposedStatus: http.StatusOK,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          nil,
		}
	}
	cookieValue := analyzer.getSessionValue(req)
	if cookieValue == "" {
		services.LogCookies(req, log.Debug()).
			Str("internalCookie", analyzer.internalSessionCookie).
			Str("externalCookie", analyzer.externalSessionCookie).
			Msgf("failed to find authentication cookies")
		return services.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         common.InvalidUserID,
			SessionID:      "",
			Error:          fmt.Errorf("session cookie not found"),
		}
	}
	sessionID := analyzer.GetSessionID(req)
	banned, userID, err := cncdb.FindBanBySession(analyzer.db, analyzer.location, sessionID, serviceName)
	if err == sql.ErrNoRows || userID == analyzer.AnonymousUserID || !userID.IsValid() {
		log.Debug().Msgf("failed to find session %s in database", sessionID)
		return services.ReqProperties{
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
	return services.ReqProperties{
		ProposedStatus: status,
		UserID:         userID,
		SessionID:      sessionID,
		Error:          err,
	}
}

func NewCNCUserAnalyzer(
	db *sql.DB,
	locaction *time.Location,
	userTableProps cncdb.UserTableProps,
	internalSessionCookie string,
	externalSessionCookie string,
	anonymousUserID common.UserID,

) *CNCUserAnalyzer {
	return &CNCUserAnalyzer{
		db:                    db,
		location:              locaction,
		userTableProps:        userTableProps,
		internalSessionCookie: internalSessionCookie,
		externalSessionCookie: externalSessionCookie,
		AnonymousUserID:       anonymousUserID,
	}
}
