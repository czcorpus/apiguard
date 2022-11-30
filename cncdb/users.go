// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"database/sql"
	"fmt"
)

type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	Affiliation string `json:"affiliation"`
}

type UsersTable struct {
	db        *sql.DB
	tableName string
}

func (users *UsersTable) UserInfo(id int) (*User, error) {
	row := users.db.QueryRow(
		fmt.Sprintf(
			"SELECT id, user, firstName, surname, email, affiliation FROM %s WHERE id = ?",
			users.tableName),
		id,
	)
	var ans User
	err := row.Scan(&ans.ID, &ans.Username, &ans.FirstName, &ans.LastName, &ans.Affiliation)
	if err == sql.ErrNoRows {
		return nil, err
	}
	return &ans, err
}

func NewUsersTable(db *sql.DB, tableName string) *UsersTable {
	return &UsersTable{
		db:        db,
		tableName: tableName,
	}
}
