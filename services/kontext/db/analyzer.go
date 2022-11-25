// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package db

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type KonTextUsersAnalyzer struct {
	db                   *sql.DB
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

func (kua *KonTextUsersAnalyzer) UserInducedResponseStatus(req *http.Request) (int, error) {
	if kua.db == nil {
		return http.StatusOK, nil
	}
	var cookieValue string
	for _, cookie := range req.Cookies() {
		if cookie.Name == kua.CNCSessionCookieName {
			cookieValue = cookie.Value
			break
		}
	}
	if cookieValue == "" {
		return http.StatusUnauthorized, fmt.Errorf("session cookie not found")
	}
	tmp := strings.SplitN(cookieValue, "-", 2)
	row := kua.db.QueryRow(
		"SELECT COUNT(*), kb.user_id "+
			"FROM kontext_user_ban AS kb "+
			"JOIN user_session AS us ON us.user_id = kb.user_id "+
			"WHERE kb.start_dt <= NOW() AND kb.end_dt > NOW() "+
			"AND kb.active = 1 AND us.selector = ?",
		tmp[0],
	)
	banned := false
	var userID int
	err := row.Scan(&banned, &userID)
	if userID == kua.AnonymousUserID {
		return http.StatusUnauthorized, nil
	}
	status := http.StatusOK
	if banned {
		status = http.StatusForbidden
	}

	return status, err
}

func NewKonTextUsersAnalyzer(
	db *sql.DB,
	usersTableName string,
	cncSessionCookieName string,
	anonymousUserID int,

) *KonTextUsersAnalyzer {
	return &KonTextUsersAnalyzer{
		db:                   db,
		UsersTableName:       usersTableName,
		CNCSessionCookieName: cncSessionCookieName,
		AnonymousUserID:      anonymousUserID,
	}
}
