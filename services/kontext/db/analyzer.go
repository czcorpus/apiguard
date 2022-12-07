// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package db

import (
	"apiguard/cncdb"
	"apiguard/services"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type KonTextUsersAnalyzer struct {
	db                   *sql.DB
	location             *time.Location
	UsersTableName       string
	CNCSessionCookieName string
	AnonymousUserID      int
}

func (kua *KonTextUsersAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	return 0, nil
}

func (kua *KonTextUsersAnalyzer) RegisterDelayLog(respDelay time.Duration) error {
	return nil // TODO
}

func (kua *KonTextUsersAnalyzer) GetSessionID(req *http.Request) string {
	cookieValue := services.GetSessionKey(req, kua.CNCSessionCookieName)
	if cookieValue == "" {
		return ""
	}
	return strings.SplitN(cookieValue, "-", 2)[0]
}

func (kua *KonTextUsersAnalyzer) UserInducedResponseStatus(req *http.Request) services.ReqProperties {
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

func NewKonTextUsersAnalyzer(
	db *sql.DB,
	locaction *time.Location,
	usersTableName string,
	cncSessionCookieName string,
	anonymousUserID int,

) *KonTextUsersAnalyzer {
	return &KonTextUsersAnalyzer{
		db:                   db,
		location:             locaction,
		UsersTableName:       usersTableName,
		CNCSessionCookieName: cncSessionCookieName,
		AnonymousUserID:      anonymousUserID,
	}
}
