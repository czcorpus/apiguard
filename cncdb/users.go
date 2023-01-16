// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"apiguard/common"
	"database/sql"
	"fmt"
)

type User struct {
	ID          common.UserID `json:"id"`
	Username    string        `json:"username"`
	FirstName   string        `json:"firstName"`
	LastName    string        `json:"lastName"`
	Email       string        `json:"email"`
	Affiliation string        `json:"affiliation"`
}

type UsersTable struct {
	db         *sql.DB
	tableProps UserTableProps
}

func (users *UsersTable) UserInfo(id common.UserID) (*User, error) {
	row := users.db.QueryRow(
		fmt.Sprintf(
			"SELECT id, %s, %s, %s, email, affiliation FROM %s WHERE id = ?",
			users.tableProps.UsernameColName,
			users.tableProps.FirstnameColName,
			users.tableProps.LastnameColName,
			users.tableProps.UserTableName),
		id,
	)
	var ans User
	err := row.Scan(
		&ans.ID,
		&ans.Username,
		&ans.FirstName,
		&ans.LastName,
		&ans.Email,
		&ans.Affiliation,
	)
	if err == sql.ErrNoRows {
		return nil, err
	}
	return &ans, err
}

func NewUsersTable(db *sql.DB, tableProps UserTableProps) *UsersTable {
	return &UsersTable{
		db:         db,
		tableProps: tableProps,
	}
}
