// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package analyzer

import (
	"apiguard/cncdb"
	"apiguard/common"
	"apiguard/services"
	"database/sql"
	"fmt"
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
	db                    *sql.DB
	location              *time.Location
	userTableProps        cncdb.UserTableProps
	internalSessionCookie string
	externalSessionCookie string
	AnonymousUserID       common.UserID
}

// CalcDelay calculates a delay user deserves. CNCUserAnalyzer
// returns 0.
func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
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

func (kua *CNCUserAnalyzer) GetInternalSessionID(req *http.Request) string {
	v := services.GetCookieValue(req, kua.internalSessionCookie)
	if v != "" {
		return strings.SplitN(v, "-", 2)[0]
	}
	return v
}

// UserInducedResponseStatus produces a HTTP response status
// proposal based on user activity.
func (kua *CNCUserAnalyzer) UserInducedResponseStatus(req *http.Request, serviceName string) services.ReqProperties {
	if kua.db == nil {
		return services.ReqProperties{
			ProposedStatus: http.StatusOK,
			UserID:         -1,
			SessionID:      "",
			Error:          nil,
		}
	}
	cookieValue := kua.getSessionValue(req)
	if cookieValue == "" {
		services.LogCookies(req, log.Debug()).
			Str("internalCookie", kua.internalSessionCookie).
			Str("externalCookie", kua.externalSessionCookie).
			Msgf("failed to find authentication cookies")
		return services.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         -1,
			SessionID:      "",
			Error:          fmt.Errorf("session cookie not found"),
		}
	}
	sessionID := kua.GetSessionID(req)
	internalSessionID := kua.GetInternalSessionID(req) // session used by internal service/API
	banned, userID, err := cncdb.FindBanBySession(kua.db, kua.location, internalSessionID, serviceName)
	if err == sql.ErrNoRows || userID == kua.AnonymousUserID {
		log.Debug().Msgf("failed to find session %s in database", sessionID)
		return services.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         -1,
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
