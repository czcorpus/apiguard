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

	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/globctx"
	"github.com/czcorpus/apiguard/session"
)

type UserFinder interface {
	FindUserBySession(sessionID session.HTTPSession) (common.UserID, error)
	GetAllowlistUsers(service string) ([]common.UserID, error)
	InvalidUserIsOK() bool
}

// --------------------------------------

type NilUserFinder struct {
}

func (uf *NilUserFinder) FindUserBySession(sessionID session.HTTPSession) (common.UserID, error) {
	return common.InvalidUserID, nil
}

func (uf *NilUserFinder) GetAllowlistUsers(service string) ([]common.UserID, error) {
	return []common.UserID{}, nil
}

func (uf *NilUserFinder) InvalidUserIsOK() bool {
	return true
}

// --------------------------------------

type SQLUserFinder struct {
	db *sql.DB
}

// FindUserBySession searches for user session in CNC database.
// In case nothing is found, `common.InvalidUserID` is returned
// (and no error). Error is returned only in case of an actual fail
// (e.g. failed db query).
func (uf *SQLUserFinder) FindUserBySession(sessionID session.HTTPSession) (common.UserID, error) {
	row := uf.db.QueryRow(
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

func (uf *SQLUserFinder) GetAllowlistUsers(service string) ([]common.UserID, error) {
	ans := make([]common.UserID, 0, 50)
	rows, err := uf.db.Query(
		"SELECT user_id FROM api_user_allowlist WHERE service_name = ?",
		service,
	)
	if err != nil {
		return []common.UserID{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID common.UserID
		err := rows.Scan(&userID)
		if err != nil {
			return []common.UserID{}, err
		}
		ans = append(ans, userID)
	}
	return ans, nil
}

func (uf *SQLUserFinder) InvalidUserIsOK() bool {
	return false
}

// --------------------------------------

func NewUserFinder(globalCtx *globctx.Context) UserFinder {
	if globalCtx.CNCDB != nil {
		return &SQLUserFinder{db: globalCtx.CNCDB}
	}
	return &NilUserFinder{}
}
