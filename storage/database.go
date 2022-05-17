// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package storage

import (
	"database/sql"
	"time"
	"wum/botwatch"
	"wum/logging"
	"wum/telemetry"

	"github.com/go-sql-driver/mysql"
)

/*

CREATE TABLE client_stats (
	id INT NOT NULL auto_increment,
	session_id VARCHAR(64) NOT NULL,
	client_ip VARCHAR(45) NOT NULL,
	cnt int NOT NULL DEFAULT 0,
	mean FLOAT NOT NULL,
	m2 FLOAT NOT NULL,
	stdev FLOAT NOT NULL,
	first_request datetime(3) NOT NULL,
	last_request datetime(3) NOT NULL,
	INDEX session_id_client_ip_idx (session_id, client_ip),
	PRIMARY KEY (id)
);

CREATE TABLE client_actions (
	id INT NOT NULL auto_increment,
	session_id VARCHAR(64),
	client_ip VARCHAR(45),
	created datetime(3) NOT NULL,
	tile_name VARCHAR(255),
	action_name VARCHAR(255),
	is_mobile tinyint NOT NULL DEFAULT 0,
	is_subquery tinyint NOT NULL DEFAULT 0,
	training_flag TINYINT, -- marks training data (1 = legit usage, 0 = bot usage, NULL - not training data)
	PRIMARY KEY (id),
	FOREIGN KEY (session_id, client_ip) REFERENCES client_stats(session_id, client_ip)
);

CREATE TABLE client_counting_rules (
	tile_name VARCHAR(63),
	action_name VARCHAR(127) NOT NULL,
	count FLOAT NOT NULL DEFAULT 1,
	tolerance FLOAT NOT NULL DEFAULT 0,
	PRIMARY KEY (tile_name, action_name)
);



*/

type MySQLAdapter struct {
	conn *sql.DB
}

func (c *MySQLAdapter) LoadStatsList(maxItems, maxAgeSecs int) ([]*botwatch.IPProcData, error) {
	if maxAgeSecs <= 0 {
		maxAgeSecs = 3600 * 24
	}
	result, err := c.conn.Query(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM client_stats
		WHERE last_request >= current_timestamp - INTERVAL ? SECOND
		ORDER BY cnt DESC LIMIT ?`, maxAgeSecs, maxItems)
	if err != nil {
		return []*botwatch.IPProcData{}, nil
	}
	ans := make([]*botwatch.IPProcData, 0, maxItems)
	for result.Next() {
		var item botwatch.IPProcData
		scanErr := result.Scan(
			&item.SessionID,
			&item.ClientIP,
			&item.Mean,
			&item.M2,
			&item.Count,
			&item.FirstAccess,
			&item.LastAccess,
		)
		if scanErr != nil {
			return []*botwatch.IPProcData{}, nil
		}
		ans = append(ans, &item)
	}
	return ans, nil
}

func (c *MySQLAdapter) LoadStats(clientIP, sessionID string, maxAgeSecs int) (*botwatch.IPProcData, error) {
	ans := c.conn.QueryRow(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM client_stats WHERE session_id = ? AND client_ip = ? AND current_timestamp - INTERVAL ? SECOND < last_request`,
		sessionID, clientIP, maxAgeSecs,
	)
	var data botwatch.IPProcData
	scanErr := ans.Scan(&data.SessionID, &data.ClientIP, &data.Mean, &data.M2, &data.Count, &data.FirstAccess, &data.LastAccess)
	if ans.Err() != nil {
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		return &botwatch.IPProcData{
			SessionID: sessionID,
			ClientIP:  clientIP,
			Count:     0,
			Mean:      0,
			M2:        0,
		}, nil

	} else if scanErr != nil {
		return nil, scanErr
	}
	return &data, nil
}

func (c *MySQLAdapter) ResetStats(data *botwatch.IPProcData) error {
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
}

func (c *MySQLAdapter) LoadIPStats(clientIP string) (*botwatch.IPProcData, error) {
	ans := c.conn.QueryRow(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM client_stats WHERE client_ip = ?`,
		clientIP,
	)
	var data botwatch.IPProcData
	scanErr := ans.Scan(&data.SessionID, &data.ClientIP, &data.Mean, &data.M2, &data.Count, &data.FirstAccess, &data.LastAccess)
	if ans.Err() != nil {
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		return &botwatch.IPProcData{
			SessionID: logging.EmptySessionIDPlaceholder,
			ClientIP:  clientIP,
			Count:     0,
			Mean:      0,
			M2:        0,
		}, nil

	} else if scanErr != nil {
		return nil, scanErr
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

func (c *MySQLAdapter) CalcStatsTelemetryDiscrepancy(clientIP, sessionID string, historySecs int) (int, error) {
	res := c.conn.QueryRow(
		`SELECT cnt
		FROM client_stats
		WHERE
			client_ip = ? AND session_id = ?
		   	AND current_timestamp - interval ? SECOND < last_request`,
		clientIP, sessionID, historySecs,
	)
	var cnt1 int
	scanErr := res.Scan(&cnt1)
	if res.Err() != nil {
		return 0, res.Err()

	} else if scanErr == sql.ErrNoRows {
		return 0, nil

	} else if scanErr != nil {
		return 0, scanErr
	}

	res = c.conn.QueryRow(
		`SELECT COUNT(*)
		FROM client_actions
		WHERE
			client_ip = ? AND session_id = ?
			AND action_name = 'MAIN_REQUEST_QUERY_RESPONSE'
		   	AND current_timestamp - interval ? SECOND < created`,
		clientIP, sessionID, historySecs,
	)
	var cnt2 int
	scanErr = res.Scan(&cnt2)
	if res.Err() != nil {
		return 0, res.Err()

	} else if scanErr == sql.ErrNoRows {
		return 0, nil

	} else if scanErr != nil {
		return 0, scanErr
	}
	return cnt1 - cnt2, nil
}

func (c *MySQLAdapter) InsertBotLikeTelemetry(clientIP, sessionID string) error {
	tx, err := c.StartTx()
	if err != nil {
		return err
	}
	t0 := time.Now()
	t1 := t0.Add(time.Duration(100) * time.Millisecond)

	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO client_actions (client_ip, session_id, action_name, created)
			VALUES (?, ?, 'MAIN_SET_TILE_RENDER_SIZE', ?)`,
			clientIP, sessionID, t0,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO client_actions (client_ip, session_id, action_name, created)
		VALUES (?, ?, 'MAIN_REQUEST_QUERY_RESPONSE', ?)`,
		clientIP, sessionID, t0,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO client_actions (client_ip, session_id, action_name, created)
			VALUES (?, ?, 'MAIN_TILE_DATA_LOADED', ?)`,
			clientIP, sessionID, t1,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO client_actions (client_ip, session_id, action_name, created)
			VALUES (?, ?, 'MAIN_TILE_PARTIAL_DATA_LOADED', ?)`,
			clientIP, sessionID, t1,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (c *MySQLAdapter) InsertTelemetry(transact *sql.Tx, data telemetry.Payload) error {
	for _, rec := range data.Telemetry {
		_, err := transact.Exec(`
			INSERT INTO client_actions (client_ip, session_id, action_name, tile_name,
				is_mobile, is_subquery, created) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rec.ClientIP, rec.SessionID, rec.ActionName, rec.TileName, rec.IsMobile,
			rec.IsSubquery, rec.Created,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *MySQLAdapter) LoadTelemetry(sessionID, clientIP string, maxAgeSecs int) ([]*telemetry.ActionRecord, error) {
	qAns, err := c.conn.Query(
		`SELECT client_ip, session_id, action_name, tile_name, is_mobile, is_subquery, created
		FROM client_actions
		WHERE client_ip = ? AND session_id = ? AND created >= current_timestamp - INTERVAL ? SECOND
		ORDER BY id ASC`,
		clientIP, sessionID, maxAgeSecs,
	)
	if err != nil {
		return []*telemetry.ActionRecord{}, err
	}
	ans := make([]*telemetry.ActionRecord, 0, 100)
	for qAns.Next() {
		var item telemetry.ActionRecord
		var tileName sql.NullString
		err := qAns.Scan(
			&item.ClientIP, &item.SessionID, &item.ActionName, &tileName,
			&item.IsMobile, &item.IsSubquery, &item.Created,
		)
		if err != nil {
			return []*telemetry.ActionRecord{}, err
		}
		if tileName.Valid {
			item.TileName = tileName.String
		}
		ans = append(ans, &item)
	}
	return ans, nil
}

func (c *MySQLAdapter) LoadCountingRules() ([]*telemetry.CountingRule, error) {
	qAns, err := c.conn.Query(
		`SELECT tile_name, action_name, count, tolerance
		FROM client_counting_rules`,
	)
	if err != nil {
		return []*telemetry.CountingRule{}, err
	}
	ans := make([]*telemetry.CountingRule, 0, 10)
	for qAns.Next() {
		var item telemetry.CountingRule
		qAns.Scan(&item.TileName, &item.ActionName, &item.Count, &item.Tolerance)
		ans = append(ans, &item)
	}
	return ans, nil
}

func (c *MySQLAdapter) getNumDeleted(tx *sql.Tx) (int, error) {
	qAns := tx.QueryRow("SELECT ROW_COUNT()")
	var numDel int
	scanErr := qAns.Scan(&numDel)
	if qAns.Err() != nil {
		return -1, qAns.Err()
	}
	if scanErr != nil {
		return -1, scanErr
	}
	return numDel, nil
}

func (c *MySQLAdapter) CleanOldData(maxAgeDays int) DataCleanupResult {
	ans := DataCleanupResult{}
	tx, err := c.StartTx()
	if err != nil {
		ans.Error = err
		return ans
	}

	_, err = tx.Exec(
		"DELETE FROM client_actions WHERE NOW() - INTERVAL ? DAY < created",
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel1, err := c.getNumDeleted(tx)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedActions = numDel1

	_, err = tx.Exec(
		"DELETE FROM client_stats WHERE NOW() - INTERVAL ? DAY < last_request",
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel2, err := c.getNumDeleted(tx)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedStats = numDel2

	err = tx.Commit()
	ans.Error = err
	return ans
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
	conf.ParseTime = true
	db, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return &MySQLAdapter{conn: db}, nil
}
