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
)

type CNCUserAnalyzer struct {
	db                   *sql.DB
	location             *time.Location
	userTableProps       cncdb.UserTableProps
	CNCSessionCookieName string
	AnonymousUserID      common.UserID
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
	banned, userID, err := cncdb.FindBanBySession(kua.db, kua.location, sessionID, serviceName)
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
	userTableProps cncdb.UserTableProps,
	cncSessionCookieName string,
	anonymousUserID common.UserID,

) *CNCUserAnalyzer {
	return &CNCUserAnalyzer{
		db:                   db,
		location:             locaction,
		userTableProps:       userTableProps,
		CNCSessionCookieName: cncSessionCookieName,
		AnonymousUserID:      anonymousUserID,
	}
}
