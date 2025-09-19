// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package guard

import (
	"database/sql"

	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard/session"
)

// FindUserBySession searches for user session in CNC database.
// In case nothing is found, `common.InvalidUserID` is returned
// (and no error). Error is returned only in case of an actual fail
// (e.g. failed db query).
func FindUserBySession(db *sql.DB, sessionID session.HTTPSession) (common.UserID, error) {
	row := db.QueryRow(
		"SELECT user_id, hashed_validator FROM user_session WHERE selector = ? LIMIT 1",
		sessionID.SrchSelector(),
	)
	var nUserID sql.NullInt64
	var nHashedValidator sql.NullString
	err := row.Scan(&nUserID, &nHashedValidator)
	if err == sql.ErrNoRows {
		return common.InvalidUserID, nil

	} else if err != nil {
		return common.InvalidUserID, err

	} else if nUserID.Valid && nHashedValidator.Valid {
		match, err := sessionID.CompareWithStoredVal(nHashedValidator.String)
		if err != nil {
			return common.InvalidUserID, nil

		} else if match {
			return common.UserID(nUserID.Int64), nil
		}

	}
	return common.InvalidUserID, nil
}
