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
