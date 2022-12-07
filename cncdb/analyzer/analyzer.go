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

func (kua *CNCUserAnalyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	return 0, nil
}

func (kua *CNCUserAnalyzer) RegisterDelayLog(respDelay time.Duration) error {
	return nil // TODO
}

func (kua *CNCUserAnalyzer) UserInducedResponseStatus(req *http.Request) (int, int, error) {
	if kua.db == nil {
		return http.StatusOK, -1, nil
	}
	cookieValue := services.GetSessionKey(req, kua.CNCSessionCookieName)
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
