// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package guard

import (
	"apiguard/common"
	"database/sql"
)

// GetAllowlistUsers returns a list of user IDs excluded from
// counting requests and limit checking.
func GetAllowlistUsers(db *sql.DB, service string) ([]common.UserID, error) {
	ans := make([]common.UserID, 0, 50)
	rows, err := db.Query(
		"SELECT user_id FROM api_user_allowlist WHERE service_name = ?",
		service,
	)
	if err != nil {
		return []common.UserID{}, err
	}
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
