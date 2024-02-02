// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package guard

import (
	"apiguard/common"
	"apiguard/session"
	"database/sql"
)

// FindUserBySession searches for user session in CNC database.
// In case nothing is found, `common.InvalidUserID` is returned
// (and no error). Error is returned only in case of an actual fail
// (e.g. failed db query).
func FindUserBySession(db *sql.DB, sessionID session.CNCSessionValue) (common.UserID, error) {
	row := db.QueryRow(
		"SELECT user_id, hashed_validator FROM user_session WHERE selector = ? LIMIT 1",
		sessionID.Selector,
	)
	var nUserID sql.NullInt64
	var nHashedValidator sql.NullString
	err := row.Scan(&nUserID, &nHashedValidator)
	if err == sql.ErrNoRows {
		return common.InvalidUserID, nil

	} else if err != nil {
		return common.InvalidUserID, err

	} else if nUserID.Valid && nHashedValidator.Valid {
		match, err := sessionID.CompareWithHash(nHashedValidator.String)
		if err != nil {
			return common.InvalidUserID, nil

		} else if match {
			return common.UserID(nUserID.Int64), nil
		}

	}
	return common.InvalidUserID, nil
}
