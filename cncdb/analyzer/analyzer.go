// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package analyzer

import (
	"apiguard/cncdb"
	"apiguard/services"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type CNCUserAnalyzer struct {
	db                   *sql.DB
	location             *time.Location
	UsersTableName       string
	CNCSessionCookieName string
	AnonymousUserID      int
}

// ValidateResponseSession checks whether the session ID stored in 'resp'
// response matches the one provided in 'req'. In case the valued
// do not match, the response 'resp' is replaced by a new one with
// status "forbidden".
// This is intended for responses stored in cache (or somewhere else) as
// normally a backend returning the response is expected to handle session
// management by itself.
func (kua *CNCUserAnalyzer) ValidateResponseSession(
	req *http.Request,
	resp services.BackendResponse,
) services.BackendResponse {
	validSess, err := resp.IsValidSession(kua.CNCSessionCookieName, kua.GetSessionID(req))
	if err != nil {
		return &services.ProxiedResponse{Err: err}
	}
	if !validSess {
		return &services.ProxiedResponse{StatusCode: http.StatusForbidden}
	}
	return resp
}

func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	return 0, nil
}

func (kua *CNCUserAnalyzer) RegisterDelayLog(respDelay time.Duration) error {
	return nil // TODO
}

func (kua *CNCUserAnalyzer) GetSessionID(req *http.Request) string {
	cookieValue := services.GetSessionKey(req, kua.CNCSessionCookieName)
	if cookieValue == "" {
		return ""
	}
	return strings.SplitN(cookieValue, "-", 2)[0]
}

func (kua *CNCUserAnalyzer) UserInducedResponseStatus(req *http.Request) services.ReqProperties {
	if kua.db == nil {
		return services.ReqProperties{
			ProposedStatus: http.StatusOK,
			UserID:         -1,
			SessionID:      "",
			Error:          nil,
		}
	}
	cookieValue := services.GetSessionKey(req, kua.CNCSessionCookieName)
	if cookieValue == "" {
		return services.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			UserID:         -1,
			SessionID:      "",
			Error:          fmt.Errorf("session cookie not found"),
		}
	}
	sessionID := kua.GetSessionID(req)
	banned, userID, err := cncdb.FindBanForSession(kua.db, kua.location, sessionID)
	if err == sql.ErrNoRows || userID == kua.AnonymousUserID {
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
	usersTableName string,
	cncSessionCookieName string,
	anonymousUserID int,

) *CNCUserAnalyzer {
	return &CNCUserAnalyzer{
		db:                   db,
		location:             locaction,
		UsersTableName:       usersTableName,
		CNCSessionCookieName: cncSessionCookieName,
		AnonymousUserID:      anonymousUserID,
	}
}
