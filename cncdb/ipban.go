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
)

func (c *DelayStats) InsertIPBan(IP net.IP, ttl int) error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}
	if ttl > 0 {
		_, err = tx.Exec(`INSERT INTO client_ip_bans (ip_address, ttl) VALUES (?, ?)`, IP.String(), ttl)

	} else {
		_, err = tx.Exec(`INSERT INTO client_ip_bans (ip_address) VALUES (?)`, IP.String())
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
	res, err = tx.Exec(`DELETE FROM client_ip_bans WHERE ip_address = ?`, IP.String())
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
	qAns := c.conn.QueryRow(
		"SELECT NOW() - INTERVAL ttl SECOND < created FROM client_ip_bans WHERE ip_address = ?",
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
