// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package guard

import (
	"apiguard/common"
	"apiguard/session"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"fmt"
)

// FindUserBySession searches for user session in CNC database.
// In case nothing is found, -1 is returned
func FindUserBySession(db *sql.DB, sessionID session.CNCSessionValue) (common.UserID, error) {
	row := db.QueryRow("SELECT user_id, hashed_validator FROM user_session WHERE selector = ? LIMIT 1", sessionID.Selector)
	var nUserID sql.NullInt64
	var nHashedValidator sql.NullString
	err := row.Scan(&nUserID, &nHashedValidator)
	if err == sql.ErrNoRows {
		return common.InvalidUserID, nil

	} else if err != nil {
		return common.InvalidUserID, err

	} else if nUserID.Valid && nHashedValidator.Valid {
		hasher := sha256.New()
		if _, err := hasher.Write([]byte(sessionID.Validator)); err != nil {
			return common.InvalidUserID, err
		}
		hashedValidator := fmt.Sprintf("%x", hasher.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(hashedValidator), []byte(nHashedValidator.String)) == 1 {
			return common.UserID(nUserID.Int64), nil
		}
	}
	return common.InvalidUserID, nil
}
