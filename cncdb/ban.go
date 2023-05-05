// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"apiguard/common"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrorUserAlreadyBannned = errors.New("user already banned")
)

type UserBan struct {
	UserID  int       `json:"userId"`
	StartDT time.Time `json:"start"`
	EndDT   time.Time `json:"end"`
	Active  int       `json:"active"`
}

func MostRecentActiveBan(db *sql.DB, loc *time.Location, userID int) (UserBan, error) {
	now := time.Now().In(loc)
	row := db.QueryRow(
		"SELECT user_id, start_dt, end_dt, active "+
			"FROM api_user_ban "+
			"WHERE start_dt <= ? AND end_dt > ? AND active = 1 AND user_id = ? "+
			"ORDER BY end_dt DESC",
		now, now, userID,
	)
	var ans UserBan
	err := row.Scan(&ans.UserID, &ans.StartDT, &ans.EndDT, &ans.Active)
	if err != nil {
		return ans, fmt.Errorf("failed to find latest user ban: %w", err)
	}
	return ans, nil
}

// BanUser
// The function returns an ID of a newly created row
func BanUser(
	db *sql.DB,
	loc *time.Location,
	userID common.UserID,
	reportID *string,
	startDate, endDate time.Time,
) (int64, error) {
	var newID int64 = -1
	tx, err := db.Begin()
	if err != nil {
		return newID, fmt.Errorf("failed to ban user: %w", err)
	}
	row := tx.QueryRow("SELECT COUNT(*) "+
		"FROM api_user_ban "+
		"WHERE "+
		"(? BETWEEN start_dt AND end_dt OR "+
		"? BETWEEN start_dt AND end_dt OR "+
		"start_dt BETWEEN ? AND ? OR "+
		"end_dt BETWEEN ? AND ?) "+
		"AND active = 1 AND user_id = ? ",
		startDate, endDate, startDate, endDate, startDate, endDate, userID)
	var numBans int
	err = row.Scan(&numBans)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			log.Error().Err(err).Msg("failed to rollback transaction")
		}
		return newID, fmt.Errorf("failed to ban user: %w", err)
	}
	if numBans > 0 {
		tx.Rollback()
		return newID, ErrorUserAlreadyBannned
	}
	var res sql.Result
	res, err = tx.Exec(
		"INSERT INTO api_user_ban (user_id, report_id, start_dt, end_dt, active) "+
			"VALUES (?, ?, ?, ?, 1)", userID, reportID, startDate, endDate)
	if err != nil {
		return newID, fmt.Errorf("failed to insert new ban: %w", err)
	}
	newID, err = res.LastInsertId()
	if err != nil {
		return newID, fmt.Errorf("failed to insert new ban: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			log.Error().Err(err).Msg("failed to rollback transaction")
		}
	}
	return newID, nil
}

// UnbanUser removes (by setting 'active=0') all bans ending in the future
// (i.e. even the ones with postponed validity).
// The function returns number of actually disabled bans
func UnbanUser(db *sql.DB, loc *time.Location, userID int) (int64, error) {
	var numBans int64 = -1
	res, err := db.Exec(
		"UPDATE api_user_ban SET active = 0 "+
			"WHERE end_dt > ? AND active = 1 AND user_id = ? ",
		time.Now().In(loc), userID,
	)
	if err != nil {
		return numBans, err
	}
	numBans, err = res.RowsAffected()
	return numBans, err
}

// FindUserBySession searches for user session in CNC database.
// In case nothing is found, -1 is returned
func FindUserBySession(db *sql.DB, sessionID string) (common.UserID, error) {
	row := db.QueryRow("SELECT user_id FROM user_session WHERE selector = ?", sessionID)
	var nUserID sql.NullInt64
	err := row.Scan(&nUserID)
	if err == sql.ErrNoRows {
		return common.InvalidUserID, nil

	} else if err != nil {
		return common.InvalidUserID, err

	} else if nUserID.Valid {
		return common.UserID(nUserID.Int64), nil
	}
	return common.InvalidUserID, nil
}

// FindBanBySession finds both userID and ban status for a defined session.
// Returned values are: (is_banned, user_id, error)
func FindBanBySession(
	db *sql.DB,
	loc *time.Location,
	sessionID string,
	serviceName string,
) (bool, common.UserID, error) {
	now := time.Now().In(loc)
	row := db.QueryRow(
		"SELECT kb.active, us.user_id, NOT ISNULL(ka.service_name) as in_allowlist "+
			"FROM user_session AS us "+
			"LEFT JOIN api_user_ban AS kb ON us.user_id = kb.user_id "+
			"  AND kb.start_dt <= ? AND kb.end_dt > ? AND kb.active = 1 "+
			"LEFT JOIN api_user_allowlist AS ka ON us.user_id = ka.user_id "+
			"  AND ka.service_name = ? "+
			"WHERE us.selector = ?",
		now, now, serviceName, sessionID)
	var banned sql.NullInt16
	var nUserID sql.NullInt64
	userID := common.InvalidUserID
	var inAllowlist bool
	err := row.Scan(&banned, &nUserID, &inAllowlist)
	if err == nil && nUserID.Valid {
		userID = common.UserID(nUserID.Int64)
	}
	if !inAllowlist && banned.Valid && banned.Int16 == 1 {
		return true, userID, err
	}
	return false, userID, err
}

func FindBanByReport(db *sql.DB, loc *time.Location, reportID string) (*UserBan, error) {
	now := time.Now().In(loc)
	row := db.QueryRow(
		"SELECT active, user_id, start_dt, end_dt "+
			"FROM api_user_ban "+
			"WHERE start_dt <= ? AND end_dt > ? AND active = 1 AND report_id = ?",
		now, now, reportID)
	fmt.Printf("SELECT active, user_id, start_dt, end_dt "+
		"FROM api_user_ban "+
		"WHERE start_dt <= '%v' AND end_dt > '%v' AND active = 1 AND report_id = '%s'",
		now, now, reportID)
	var ans UserBan
	err := row.Scan(&ans.Active, &ans.UserID, &ans.StartDT, &ans.EndDT)
	fmt.Println("ERR: ", err)
	fmt.Println("ANS: ", ans)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ans, err
}
