// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package db

import (
	"apiguard/cncdb"
	"apiguard/services/kontext"
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

func (kua *KonTextUsersAnalyzer) UserInducedResponseStatus(req *http.Request) (int, int, error) {
	if kua.db == nil {
		return http.StatusOK, -1, nil
	}
	cookieValue := kontext.GetSessionKey(req, kua.CNCSessionCookieName)
	if cookieValue == "" {
		return http.StatusUnauthorized, -1, fmt.Errorf("session cookie not found")
	}
	tmp := strings.SplitN(cookieValue, "-", 2)
	banned, userID, err := cncdb.FindBanForSession(kua.db, kua.location, tmp[0])
	if err == sql.ErrNoRows || userID == kua.AnonymousUserID {
		return http.StatusUnauthorized, -1, nil
	}
	status := http.StatusOK
	if banned {
		status = http.StatusForbidden
	}
	return status, userID, err
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
