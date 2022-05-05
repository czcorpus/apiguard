// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package storage

import (
	"database/sql"
	"fmt"
	"time"
	"wum/botwatch"
	"wum/telemetry"

	"github.com/go-sql-driver/mysql"
)

/*

CREATE TABLE client_stats (
	session_id VARCHAR(64) NOT NULL,
	client_ip VARCHAR(45) NOT NULL,
	cnt int NOT NULL DEFAULT 0,
	mean FLOAT NOT NULL,
	m2 FLOAT NOT NULL,
	stdev FLOAT NOT NULL,
	first_request datetime NOT NULL,
	last_request datetime NOT NULL,
	PRIMARY KEY (session_id, client_ip)
);

CREATE TABLE client_actions (
	session_id VARCHAR(64),
	client_ip VARCHAR(45),
	created datetime NOT NULL,
	tile_name VARCHAR(255),
	action_name VARCHAR(255),
	is_mobile tinyint NOT NULL DEFAULT 0,
	is_subquery tinyint NOT NULL DEFAULT 0,
	PRIMARY KEY (session_id, client_ip, created),
	FOREIGN KEY (session_id, client_ip) REFERENCES client_stats(session_id, client_ip)
);



*/

type Conf struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type MySQLAdapter struct {
	conn *sql.DB
}

func (c *MySQLAdapter) LoadStats(clientIP, sessionID string) (*botwatch.IPProcData, error) {
	ans := c.conn.QueryRow(
		`SELECT session_id, client_ip, mean, m2, cnt, stdev, first_request, last_request
		FROM client_stats WHERE session_id = ? AND client_ip = ?`,
		sessionID, clientIP,
	)
	var data botwatch.IPProcData
	scanErr := ans.Scan(&data.SessionID, &data.ClientIP, &data.Mean, &data.M2, &data.Count, &data.FirstAccess, &data.LastAccess)
	if ans.Err() != nil {
		fmt.Println("RETURNING NIL____!!!!")
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		fmt.Println("XXXX_______ NEW")
		return &botwatch.IPProcData{
			SessionID: sessionID,
			ClientIP:  clientIP,
			Count:     0,
			Mean:      0,
			M2:        0,
		}, nil
	}
	return &data, nil
}

func (c *MySQLAdapter) UpdateStats(
	data *botwatch.IPProcData,
) error {
	curr := c.conn.QueryRow(
		`SELECT COUNT(*) FROM client_stats WHERE client_ip = ? AND session_id = ?`,
		data.ClientIP, data.SessionID)
	var cnt int
	scanErr := curr.Scan(&cnt)
	fmt.Println("++++++++++++++++ UPDATE STATS, exists? ", cnt, data.ClientIP, data.SessionID)
	if curr.Err() != nil {
		return curr.Err()

	} else if scanErr != nil {
		return scanErr

	} else if cnt == 0 {
		tx, err := c.StartTx()
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			`INSERT INTO client_stats (session_id, client_ip, mean, m2, cnt, stdev, first_request, last_request)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			data.SessionID, data.ClientIP, data.Mean, data.M2, data.Count, data.Stdev(),
			data.FirstAccess, data.LastAccess)
		if err != nil {
			return err
		}
		err = tx.Commit()
		return err

	} else {
		tx, err := c.StartTx()
		if err != nil {
			return err
		}
		_, err = c.conn.Exec(
			`UPDATE client_stats SET mean = ?, m2 = ?, cnt = ?, stdev = ?, first_request = ?,
			last_request = ? WHERE session_id = ? AND client_ip = ?`,
			data.Mean, data.M2, data.Count, data.Stdev(), data.FirstAccess, data.LastAccess,
			data.SessionID, data.ClientIP,
		)
		if err != nil {
			return err
		}
		err = tx.Commit()
		return err
	}
}

func (c *MySQLAdapter) InsertTelemetry(
	transact *sql.Tx,
	sessionID string,
	clientIP string,
	data telemetry.Payload,
) error {
	for _, rec := range data.Telemetry {
		tt := time.UnixMilli(rec.TimestampMS)
		_, err := transact.Exec(`
			INSERT INTO user_actions (client_ip, session_id, user_action, tile_name,
				is_mobile, is_subquery, created) VALUES (?, ?, ?, ?, ?, ?)`,
			clientIP, sessionID, rec.ActionName, rec.TileName, rec.IsMobile, rec.IsSubquery, tt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *MySQLAdapter) StartTx() (*sql.Tx, error) {
	return c.conn.Begin()
}

func (c *MySQLAdapter) CommitTx(transact *sql.Tx) error {
	return transact.Commit()
}

func (c *MySQLAdapter) RollbackTx(transact *sql.Tx) error {
	return transact.Rollback()
}

func NewMySQLAdapter(host, user, pass, dbName string) (*MySQLAdapter, error) {
	conf := mysql.NewConfig()
	conf.Net = "tcp"
	conf.Addr = host
	conf.User = user
	conf.Passwd = pass
	conf.DBName = dbName
	db, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return &MySQLAdapter{conn: db}, nil
}
