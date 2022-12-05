// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"
)

func (c *DelayStats) InsertIPBan(IP net.IP, ttl int) error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}
	now := time.Now()
	if c.location != nil {
		now = now.In(c.location)
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

func (c *DelayStats) RemoveIPBan(IP net.IP) error {
	tx, err := c.conn.Begin()
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

func (c *DelayStats) TestIPBan(IP net.IP) (bool, error) {
	now := time.Now()
	if c.location != nil {
		now = now.In(c.location)
	}
	qAns := c.conn.QueryRow(
		"SELECT ? < end_dt FROM api_ip_ban WHERE ip_address = ? AND active = 1",
		now,
		IP.String(),
	)
	var isBanned bool
	scanErr := qAns.Scan(&isBanned)
	if scanErr == sql.ErrNoRows {
		return false, nil

	} else if scanErr != nil {
		return false, scanErr
	}
	if qAns.Err() != nil {
		return false, qAns.Err()
	}
	return isBanned, nil
}
