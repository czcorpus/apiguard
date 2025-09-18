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
	"fmt"
	"net"
	"strings"
	"time"
)

func InsertIPBan(db *sql.DB, IP net.IP, ttl int, loc *time.Location) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	now := time.Now()
	if loc != nil {
		now = now.In(loc)
	}
	if ttl > 0 {
		end_dt := now.Add(time.Duration(ttl) * time.Second)
		_, err = tx.Exec(`INSERT INTO api_ip_ban (ip_address, start_dt, end_dt) VALUES (?, ?, ?)`, IP.String(), now, end_dt)

	} else {
		end_dt := now.Add(time.Duration(86400) * time.Second)
		_, err = tx.Exec(`INSERT INTO api_ip_ban (ip_address, start_dt, end_dt) VALUES (?, ?, ?)`, IP.String(), now, end_dt)
	}
	if err != nil {
		if strings.HasPrefix(err.Error(), "Error 1062: Duplicate entry") {
			return fmt.Errorf("failed to insert ban - address %s already banned", IP.String())
		}
		return err
	}
	err = tx.Commit()
	return err
}

func RemoveIPBan(db *sql.DB, IP net.IP) error {
	tx, err := db.Begin()
	if err != nil {
		tx.Rollback()
		return err
	}
	var res sql.Result
	res, err = tx.Exec(`DELETE FROM api_ip_ban WHERE ip_address = ?`, IP.String())
	if err != nil {
		tx.Rollback()
		return err
	}

	numDel, err := res.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}
	if numDel == 0 {
		tx.Rollback()
		return fmt.Errorf("cannot unban ip %s - address not banned", IP.String())
	}
	err = tx.Commit()
	return err
}
