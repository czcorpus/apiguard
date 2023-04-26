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
	"apiguard/services/backend"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CNCUserAnalyzer provides access to user request and is able
// too access CNC session database to evaluate user permissions.
//
// It supports CookieMapping for cases where APIguard client
// (e.g. WaG, KonText) needs a different cookie than the API itself.
// E.g. WaG --- [cookie1] --> APIGuard --> [cookie2] --> API with
// the values of cookie1 and cookie2 being the same.
type CNCUserAnalyzer struct {
	db                   *sql.DB
	location             *time.Location
	userTableProps       cncdb.UserTableProps
	CNCSessionCookieName string
	CookieMapping        backend.CookieMapping
	AnonymousUserID      common.UserID
}

// CalcDelay calculates a delay user deserves. CNCUserAnalyzer
// returns 0.
func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	return 0, nil
}

func (kua *CNCUserAnalyzer) LogAppliedDelay(respDelay time.Duration) error {
	return nil // TODO
}

// GetSessionID extracts relevant user session ID from the provided Request.
func (kua *CNCUserAnalyzer) GetSessionID(req *http.Request) string {
	mappedSessionCookieName := kua.getTrueSessionCookieName(kua.CNCSessionCookieName)
	cookieValue := services.GetSessionKey(req, mappedSessionCookieName)
	if cookieValue == "" {
		return ""
	}
	return strings.SplitN(cookieValue, "-", 2)[0]
}

// getTrueSessionCookieName for a required cookie name obtain a "true" name
// based on possible cookie mapping. I.e. we ask for an "external" cookie
// (the name used by the API) but we return the name used by us and our web
// application (e.g. WaG).
func (kua *CNCUserAnalyzer) getTrueSessionCookieName(declaredName string) string {
	mapped, ok := kua.CookieMapping.KeyOfValue(declaredName)
	if ok {
		return mapped
	}
	return declaredName
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
	mappedSessionCookieName := kua.getTrueSessionCookieName(kua.CNCSessionCookieName)
	cookieValue := services.GetSessionKey(req, mappedSessionCookieName)
	if cookieValue == "" {
		services.LogCookies(req, log.Debug()).
			Msgf("failed to find cookie %s", mappedSessionCookieName)
		return services.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         -1,
			SessionID:      "",
			Error:          fmt.Errorf("session cookie not found"),
		}
	}
	sessionID := kua.GetSessionID(req)
	banned, userID, err := cncdb.FindBanBySession(kua.db, kua.location, sessionID, serviceName)
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
	cncSessionCookieName string,
	cookieMapping backend.CookieMapping,
	anonymousUserID common.UserID,

) *CNCUserAnalyzer {
	return &CNCUserAnalyzer{
		db:                   db,
		location:             locaction,
		userTableProps:       userTableProps,
		CNCSessionCookieName: cncSessionCookieName,
		CookieMapping:        cookieMapping,
		AnonymousUserID:      anonymousUserID,
	}
}
