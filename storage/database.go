// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package storage

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"
	"wum/botwatch"
	"wum/telemetry"

	"github.com/rs/zerolog/log"

	"github.com/go-sql-driver/mysql"
)

/*

CREATE TABLE client_stats (
	id INT NOT NULL auto_increment,
	session_id VARCHAR(64),
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
	PRIMARY KEY (id)
);

CREATE TABLE client_counting_rules (
	tile_name VARCHAR(63),
	action_name VARCHAR(127) NOT NULL,
	count FLOAT NOT NULL DEFAULT 1,
	tolerance FLOAT NOT NULL DEFAULT 0,
	PRIMARY KEY (tile_name, action_name)
);

CREATE TABLE client_bans (
	ip_address VARCHAR(15) NOT NULL,
	ttl int NOT NULL DEFAULT 86400,
	created datetime NOT NULL DEFAULT NOW(),
	PRIMARY KEY (ip_address)
);

*/

func string2NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

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
		var sessionID sql.NullString
		scanErr := result.Scan(
			&sessionID,
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
		if sessionID.Valid {
			item.SessionID = sessionID.String
		}
		ans = append(ans, &item)
	}
	return ans, nil
}

// LoadStats loads statistics for a specified IP and sessionID. In case nothing is found,
// a new record is inserted.
func (c *MySQLAdapter) LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*botwatch.IPProcData, error) {
	tx, err := c.StartTx()
	if err != nil {
		return nil, err
	}
	ans := c.conn.QueryRow(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM client_stats
		WHERE (session_id = ? OR session_id IS NULL AND ? IS NULL) AND client_ip = ? AND
		current_timestamp - INTERVAL ? SECOND < last_request`,
		string2NullString(sessionID), string2NullString(sessionID), clientIP, maxAgeSecs,
	)
	var data botwatch.IPProcData
	var dbSessionID sql.NullString
	scanErr := ans.Scan(&dbSessionID, &data.ClientIP, &data.Mean, &data.M2, &data.Count, &data.FirstAccess, &data.LastAccess)
	if ans.Err() != nil {
		tx.Rollback()
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		dt := time.Now()
		newData := &botwatch.IPProcData{
			SessionID:   sessionID,
			ClientIP:    clientIP,
			Count:       0,
			Mean:        0,
			M2:          0,
			FirstAccess: dt,
			LastAccess:  dt,
		}
		if insertIfNone {
			c.resetStats(tx, newData)
			err := tx.Commit()
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			log.Debug().Msgf(
				"no stats found for session %s (ip %s) - new record inserted to DB",
				sessionID, clientIP,
			)
		}
		return newData, nil

	} else if scanErr != nil {
		tx.Rollback()
		return nil, scanErr
	}
	if dbSessionID.Valid {
		data.SessionID = dbSessionID.String
	}
	return &data, nil
}

func (c *MySQLAdapter) ResetStats(data *botwatch.IPProcData) error {
	tx, err := c.StartTx()
	if err != nil {
		return err
	}
	err = c.resetStats(tx, data)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	return err
}

func (c *MySQLAdapter) resetStats(tx *sql.Tx, data *botwatch.IPProcData) error {
	ns := string2NullString(data.SessionID)
	_, err := tx.Exec(
		`INSERT INTO client_stats (session_id, client_ip, mean, m2, cnt, stdev, first_request, last_request)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ns, data.ClientIP, data.Mean, data.M2, data.Count, data.Stdev(),
		data.FirstAccess, data.LastAccess)
	if err != nil {
		return err
	}
	return nil
}

func (c *MySQLAdapter) LoadIPStats(clientIP string, maxAgeSecs int) (*botwatch.IPAggData, error) {
	ans := c.conn.QueryRow(
		`SELECT cs.client_ip, SUM(cs.mean), SUM(cs.m2), SUM(cs.cnt),
		 MIN(cs.first_request), MAX(cs.last_request)
		FROM client_stats AS cs
		LEFT JOIN client_actions AS ca ON cs.client_ip = ca.client_ip AND cs.session_id = ca.session_id
		WHERE cs.client_ip = ? AND ca.id IS NULL
		AND current_timestamp - INTERVAL ? SECOND < cs.last_request
		GROUP BY cs.client_ip `,
		clientIP, maxAgeSecs,
	)
	var data botwatch.IPAggData
	scanErr := ans.Scan(&data.ClientIP, &data.Mean, &data.M2, &data.Count, &data.FirstAccess, &data.LastAccess)
	if ans.Err() != nil {
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		return &botwatch.IPAggData{
			ClientIP: clientIP,
			Count:    0,
			Mean:     0,
			M2:       0,
		}, nil

	} else if scanErr != nil {
		return nil, scanErr
	}
	return &data, nil
}

func (c *MySQLAdapter) GetSessionIP(sessionID string) (net.IP, error) {
	ns := string2NullString(sessionID)
	ans := c.conn.QueryRow(
		`SELECT client_ip
		FROM client_stats
		WHERE (session_id = ? OR session_id IS NULL AND ? IS NULL)`,
		ns, ns,
	)
	var ipStr string
	scanErr := ans.Scan(&ipStr)
	if ans.Err() != nil {
		return nil, ans.Err()
	}
	if scanErr == sql.ErrNoRows {
		return nil, nil
	}
	return net.ParseIP(ipStr), nil
}

func (c *MySQLAdapter) UpdateStats(
	data *botwatch.IPProcData,
) error {
	ns := string2NullString(data.SessionID)
	curr := c.conn.QueryRow(
		`SELECT COUNT(*)
		FROM client_stats
		WHERE client_ip = ? AND (session_id = ? OR session_id IS NULL AND ? IS NULL)`,
		data.ClientIP, ns, ns)
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
			ns, data.ClientIP, data.Mean, data.M2, data.Count, data.Stdev(),
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
			last_request = ?
			WHERE id = (
				SELECT q2.maxid FROM (SELECT MAX(id) AS maxid FROM client_stats
					WHERE (session_id = ? OR session_id IS NULL AND ? IS NULL) AND client_ip = ?) AS q2
			)`,
			data.Mean, data.M2, data.Count, data.Stdev(), data.FirstAccess, data.LastAccess,
			string2NullString(data.SessionID), string2NullString(data.SessionID), data.ClientIP,
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
			client_ip = ? AND (session_id = ? OR session_id IS NULL AND ? IS NULL)
		   	AND current_timestamp - interval ? SECOND < last_request`,
		clientIP, string2NullString(sessionID), string2NullString(sessionID), historySecs,
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
			client_ip = ? AND (session_id = ? OR session_id IS NULL AND ? IS NULL)
			AND action_name = 'MAIN_REQUEST_QUERY_RESPONSE'
		   	AND current_timestamp - interval ? SECOND < created`,
		clientIP, string2NullString(sessionID), string2NullString(sessionID), historySecs,
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
			clientIP, string2NullString(sessionID), t0,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO client_actions (client_ip, session_id, action_name, created)
		VALUES (?, ?, 'MAIN_REQUEST_QUERY_RESPONSE', ?)`,
		clientIP, string2NullString(sessionID), t0,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO client_actions (client_ip, session_id, action_name, created)
			VALUES (?, ?, 'MAIN_TILE_DATA_LOADED', ?)`,
			clientIP, string2NullString(sessionID), t1,
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
			clientIP, string2NullString(sessionID), t1,
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
			rec.Client.IP, string2NullString(rec.Client.SessionID), rec.ActionName, rec.TileName,
			rec.IsMobile, rec.IsSubquery, rec.Created,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *MySQLAdapter) exportTelemetryRows(rows *sql.Rows) ([]*telemetry.ActionRecord, error) {
	ans := make([]*telemetry.ActionRecord, 0, 100)
	for rows.Next() {
		var item telemetry.ActionRecord
		var sessionID sql.NullString
		var tileName sql.NullString
		var trainingFlag sql.NullInt16
		err := rows.Scan(
			&item.Client.IP, &sessionID, &item.ActionName, &tileName,
			&item.IsMobile, &item.IsSubquery, &item.Created, &trainingFlag,
		)
		if err != nil {
			return []*telemetry.ActionRecord{}, err
		}
		if sessionID.Valid {
			item.Client.SessionID = sessionID.String
		}
		if tileName.Valid {
			item.TileName = tileName.String
		}
		if trainingFlag.Valid {
			item.TrainingFlag = int(trainingFlag.Int16)

		} else {
			item.TrainingFlag = -1
		}
		ans = append(ans, &item)
	}
	return ans, nil
}

// FindLearningClients finds all the clients involved in telemetry data during a specified
// time interval (maxAgeSecs is including, minAgeSecs is excluding)
func (c *MySQLAdapter) FindLearningClients(maxAgeSecs int, minAgeSecs int) ([]*telemetry.Client, error) {
	rows, err := c.conn.Query(
		`SELECT DISTINCT client_ip, session_id
		FROM client_actions
		WHERE
			created >= current_timestamp - INTERVAL ? SECOND AND
			created < current_timestamp - INTERVAL ? SECOND AND
			training_flag IS NOT NULL
		ORDER BY id ASC`,
		maxAgeSecs,
		minAgeSecs,
	)
	if err != nil {
		return []*telemetry.Client{}, err
	}
	ans := make([]*telemetry.Client, 0, 100)
	for rows.Next() {
		var sessionID sql.NullString
		var client telemetry.Client
		err := rows.Scan(&client.IP, &sessionID)
		if err != nil {
			return []*telemetry.Client{}, err
		}
		if sessionID.Valid {
			client.SessionID = sessionID.String
		}
		ans = append(ans, &client)
	}
	return ans, nil
}

// LoadClient loads telemetry for a defined client and time limit
func (c *MySQLAdapter) LoadClientTelemetry(
	sessionID, clientIP string,
	maxAgeSecs, minAgeSecs int,
) ([]*telemetry.ActionRecord, error) {
	rows, err := c.conn.Query(
		`SELECT client_ip, session_id, action_name, tile_name, is_mobile, is_subquery, created, training_flag
		FROM client_actions
		WHERE
			client_ip = ? AND
			(session_id = ? OR session_id IS NULL AND ? IS NULL) AND
			created >= current_timestamp - INTERVAL ? SECOND AND
			created < current_timestamp - INTERVAL ? SECOND
		ORDER BY id ASC`,
		clientIP, string2NullString(sessionID), string2NullString(sessionID), maxAgeSecs, minAgeSecs,
	)
	if err != nil {
		return []*telemetry.ActionRecord{}, err
	}
	x, err := c.exportTelemetryRows(rows)
	return x, err
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

func (c *MySQLAdapter) getNumAffected(tx *sql.Tx) (int, error) {
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
		`DELETE FROM client_actions
		WHERE
			NOW() - INTERVAL ? DAY > created AND
			training_flag IS NULL`,
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel1, err := c.getNumAffected(tx)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedActions = numDel1

	_, err = tx.Exec(
		"DELETE FROM client_stats WHERE NOW() - INTERVAL ? DAY > last_request",
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel2, err := c.getNumAffected(tx)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedStats = numDel2

	_, err = tx.Exec(
		"DELETE FROM client_bans WHERE NOW() - INTERVAL ttl SECOND > created",
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel3, err := c.getNumAffected(tx)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedBans = numDel3

	err = tx.Commit()
	ans.Error = err
	return ans
}

func (c *MySQLAdapter) InsertBan(IP net.IP, ttl int) error {
	tx, err := c.StartTx()
	if err != nil {
		return err
	}
	if ttl > 0 {
		_, err = tx.Exec(`INSERT INTO client_bans (ip_address, ttl) VALUES (?, ?)`, IP.String(), ttl)

	} else {
		_, err = tx.Exec(`INSERT INTO client_bans (ip_address) VALUES (?)`, IP.String())
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

func (c *MySQLAdapter) RemoveBan(IP net.IP) error {
	tx, err := c.StartTx()
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec(`DELETE FROM client_bans WHERE ip_address = ?`, IP.String())
	if err != nil {
		tx.Rollback()
		return err
	}

	numDel, err := c.getNumAffected(tx)
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

func (c *MySQLAdapter) TestIPBan(IP net.IP) (bool, error) {
	qAns := c.conn.QueryRow(
		"SELECT NOW() - INTERVAL ttl SECOND < created FROM client_bans WHERE ip_address = ?",
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
	conf.Loc = time.Local
	db, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return &MySQLAdapter{conn: db}, nil
}
