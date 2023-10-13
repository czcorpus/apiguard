// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"apiguard/botwatch"
	"apiguard/cncdb/rdelay"
	"apiguard/services"
	"apiguard/services/telemetry"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

/*

CREATE TABLE apiguard_client_stats (
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

CREATE TABLE apiguard_client_actions (
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

CREATE TABLE apiguard_client_counting_rules (
	tile_name VARCHAR(63),
	action_name VARCHAR(127) NOT NULL,
	count FLOAT NOT NULL DEFAULT 1,
	tolerance FLOAT NOT NULL DEFAULT 0,
	PRIMARY KEY (tile_name, action_name)
);

CREATE TABLE api_ip_ban (
	ip_address VARCHAR(45) NOT NULL,
	start_dt DATETIME NOT NULL DEFAULT NOW(),
	end_dt DATETIME NOT NULL,
	active TINYINT NOT NULL DEFAULT 1, -- so we can disable it any time and also to archive records
	PRIMARY KEY (ip_address)
);

CREATE TABLE apiguard_delay_log (
	id INT NOT NULL auto_increment,
	client_ip VARCHAR(45) NOT NULL,
	created datetime NOT NULL DEFAULT NOW(),
	delay FLOAT NOT NULL,
	is_ban TINYINT NOT NULL,
	PRIMARY KEY (id)
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

type DelayStats struct {
	conn     *sql.DB
	location *time.Location
}

func (c *DelayStats) LoadStatsList(maxItems, maxAgeSecs int) ([]*botwatch.IPProcData, error) {
	if maxAgeSecs <= 0 {
		maxAgeSecs = 3600 * 24
	}
	result, err := c.conn.Query(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM apiguard_client_stats
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
func (c *DelayStats) LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*botwatch.IPProcData, error) {
	tx, err := c.StartTx()
	if err != nil {
		return nil, err
	}
	ans := c.conn.QueryRow(
		`SELECT session_id, client_ip, mean, m2, cnt, first_request, last_request
		FROM apiguard_client_stats
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

func (c *DelayStats) ResetStats(data *botwatch.IPProcData) error {
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

func (c *DelayStats) resetStats(tx *sql.Tx, data *botwatch.IPProcData) error {
	ns := string2NullString(data.SessionID)
	_, err := tx.Exec(
		`INSERT INTO apiguard_client_stats (session_id, client_ip, mean, m2, cnt, stdev, first_request, last_request)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ns, data.ClientIP, data.Mean, data.M2, data.Count, data.Stdev(),
		data.FirstAccess, data.LastAccess)
	if err != nil {
		return err
	}
	return nil
}

func (c *DelayStats) LoadIPStats(clientIP string, maxAgeSecs int) (*botwatch.IPAggData, error) {
	ans := c.conn.QueryRow(
		`SELECT cs.client_ip, SUM(cs.mean), SUM(cs.m2), SUM(cs.cnt),
		 MIN(cs.first_request), MAX(cs.last_request)
		FROM apiguard_client_stats AS cs
		LEFT JOIN apiguard_client_actions AS ca ON cs.client_ip = ca.client_ip AND cs.session_id = ca.session_id
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

func (c *DelayStats) GetSessionIP(sessionID string) (net.IP, error) {
	ns := string2NullString(sessionID)
	ans := c.conn.QueryRow(
		`SELECT client_ip
		FROM apiguard_client_stats
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

func (c *DelayStats) UpdateStats(
	data *botwatch.IPProcData,
) error {
	ns := string2NullString(data.SessionID)
	curr := c.conn.QueryRow(
		`SELECT COUNT(*)
		FROM apiguard_client_stats
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
			`INSERT INTO apiguard_client_stats (session_id, client_ip, mean, m2, cnt, stdev, first_request, last_request)
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
			`UPDATE apiguard_client_stats SET mean = ?, m2 = ?, cnt = ?, stdev = ?, first_request = ?,
			last_request = ?
			WHERE id = (
				SELECT q2.maxid FROM (SELECT MAX(id) AS maxid FROM apiguard_client_stats
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

func (c *DelayStats) CalcStatsTelemetryDiscrepancy(clientIP, sessionID string, historySecs int) (int, error) {
	res := c.conn.QueryRow(
		`SELECT cnt
		FROM apiguard_client_stats
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
		FROM apiguard_client_actions
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

func (c *DelayStats) InsertBotLikeTelemetry(clientIP, sessionID string) error {
	tx, err := c.StartTx()
	if err != nil {
		return err
	}
	t0 := time.Now()
	t1 := t0.Add(time.Duration(100) * time.Millisecond)

	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO apiguard_client_actions (client_ip, session_id, action_name, created)
			VALUES (?, ?, 'MAIN_SET_TILE_RENDER_SIZE', ?)`,
			clientIP, string2NullString(sessionID), t0,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO apiguard_client_actions (client_ip, session_id, action_name, created)
		VALUES (?, ?, 'MAIN_REQUEST_QUERY_RESPONSE', ?)`,
		clientIP, string2NullString(sessionID), t0,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := 0; i < 3; i++ {
		_, err = tx.Exec(`
			INSERT INTO apiguard_client_actions (client_ip, session_id, action_name, created)
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
			INSERT INTO apiguard_client_actions (client_ip, session_id, action_name, created)
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

func (c *DelayStats) InsertTelemetry(transact *sql.Tx, data telemetry.Payload) error {
	for _, rec := range data.Telemetry {
		_, err := transact.Exec(`
			INSERT INTO apiguard_client_actions (client_ip, session_id, action_name, tile_name,
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

func (c *DelayStats) exportTelemetryRows(rows *sql.Rows) ([]*telemetry.ActionRecord, error) {
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
func (c *DelayStats) FindLearningClients(maxAgeSecs int, minAgeSecs int) ([]*telemetry.Client, error) {
	rows, err := c.conn.Query(
		`SELECT DISTINCT client_ip, session_id
		FROM apiguard_client_actions
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
func (c *DelayStats) LoadClientTelemetry(
	sessionID, clientIP string,
	maxAgeSecs, minAgeSecs int,
) ([]*telemetry.ActionRecord, error) {
	rows, err := c.conn.Query(
		`SELECT client_ip, session_id, action_name, tile_name, is_mobile, is_subquery, created, training_flag
		FROM apiguard_client_actions
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

func (c *DelayStats) LoadCountingRules() ([]*telemetry.CountingRule, error) {
	qAns, err := c.conn.Query(
		`SELECT tile_name, action_name, count, tolerance
		FROM apiguard_client_counting_rules`,
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

func (c *DelayStats) CleanOldData(maxAgeDays int) rdelay.DataCleanupResult {
	ans := rdelay.DataCleanupResult{}
	tx, err := c.StartTx()
	if err != nil {
		ans.Error = err
		return ans
	}
	var res sql.Result
	res, err = tx.Exec(
		`DELETE FROM apiguard_client_actions
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
	numDel1, err := res.RowsAffected()
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedActions = numDel1

	res, err = tx.Exec(
		"DELETE FROM apiguard_client_stats WHERE NOW() - INTERVAL ? DAY > last_request",
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel2, err := res.RowsAffected()
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedStats = numDel2

	res, err = tx.Exec(
		"DELETE FROM api_ip_ban WHERE NOW() > end_dt",
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel3, err := res.RowsAffected()
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedBans = numDel3

	res, err = tx.Exec(
		"DELETE FROM apiguard_delay_log WHERE NOW() - INTERVAL ? DAY > created",
		maxAgeDays,
	)
	if err != nil {
		tx.Rollback()
		ans.Error = err
		return ans
	}
	numDel4, err := res.RowsAffected()
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.NumDeletedDelayLogs = numDel4

	err = tx.Commit()
	ans.Error = err
	return ans
}

func (c *DelayStats) LogAppliedDelay(delayInfo services.DelayInfo, clientIP string) error {
	tx, err := c.StartTx()
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"INSERT INTO apiguard_delay_log (client_ip, delay, is_ban) VALUES (?, ?, ?)",
		clientIP,
		delayInfo.Delay.Seconds(),
		delayInfo.IsBan,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

type delayLogsHistogram struct {
	OldestRecord *time.Time     `json:"oldestRecord"`
	BinWidth     float64        `json:"binWidth"`
	OtherLimit   float64        `json:"otherLimit"`
	Data         map[string]int `json:"data"`
}

func (c *DelayStats) AnalyzeDelayLog(binWidth float64, otherLimit float64) (*delayLogsHistogram, error) {
	histogram := delayLogsHistogram{
		OldestRecord: nil,
		BinWidth:     binWidth,
		OtherLimit:   otherLimit,
		Data:         make(map[string]int),
	}
	rows, err := c.conn.Query(fmt.Sprintf(`
		SELECT
			CONVERT(LEAST(FLOOR(delay/%f)*%f, %f), CHAR) AS delay_bin,
			COUNT(*) as count
		FROM apiguard_delay_log
		WHERE is_ban = 0
		GROUP BY delay_bin
		ORDER BY delay_bin
	`, binWidth, binWidth, otherLimit))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var delayBin string
		var count int
		err := rows.Scan(&delayBin, &count)
		if err != nil {
			return nil, err
		}
		histogram.Data[delayBin] = count
	}
	if len(histogram.Data) > 0 {
		var oldestRecord time.Time
		row := c.conn.QueryRow(`
			SELECT created
			FROM apiguard_delay_log
			WHERE is_ban = 0
			ORDER BY created
			LIMIT 1
		`)
		row.Scan(&oldestRecord)
		histogram.OldestRecord = &oldestRecord
	}
	return &histogram, nil
}

type banRow struct {
	ClientIP string `json:"clientIp"`
	Bans     int    `json:"bans"`
}

func (c *DelayStats) AnalyzeBans(timeAgo time.Duration) ([]banRow, error) {
	bans := make([]banRow, 0, 100)
	rows, err := c.conn.Query(`
		SELECT
			client_ip,
			COUNT(*) as ban_count
		FROM apiguard_delay_log
		WHERE is_ban = 1 AND created >= current_timestamp - INTERVAL ? SECOND
		GROUP BY client_ip
		`,
		timeAgo.Seconds(),
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		d := banRow{}
		err := rows.Scan(&d.ClientIP, &d.Bans)
		if err != nil {
			return nil, err
		}
		bans = append(bans, d)
	}
	return bans, nil
}

func (c *DelayStats) StartTx() (*sql.Tx, error) {
	return c.conn.Begin()
}

func (c *DelayStats) CommitTx(transact *sql.Tx) error {
	return transact.Commit()
}

func (c *DelayStats) RollbackTx(transact *sql.Tx) error {
	return transact.Rollback()
}

func NewDelayStats(db *sql.DB, location *time.Location) *DelayStats {
	return &DelayStats{conn: db, location: location}
}
