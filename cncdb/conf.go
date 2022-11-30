// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cncdb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
)

/*
CREATE TABLE user_session (
  id int(11) NOT NULL AUTO_INCREMENT,
  selector char(12),
  hashed_validator char(64),
  user_id int(11),
  last_active_on varchar(25),
  internal_verified tinyint(1) NOT NULL DEFAULT 0,
  data text,
  PRIMARY KEY (id),
  UNIQUE KEY uc_user_session_selector (selector),
  KEY fk_user_session_user_id (user_id),
  CONSTRAINT fk_user_session_user_id FOREIGN KEY (user_id) REFERENCES user(id)
) ENGINE=InnoDB

CREATE TABLE api_user_ban (
	id INTEGER NOT NULL auto_increment,
	user_id INTEGER NOT NULL,
	start_dt DATETIME NOT NULL,
	end_dt DATETIME NOT NULL,
	active TINYINT NOT NULL DEFAULT 1, -- so we can disable it any time and also to archive records
	PRIMARY KEY (id),
	FOREIGN KEY (user_id) REFERENCES kontext_user(id)
)

-- privileges:

create user 'apiguard'@'192.168.1.%' identified by '******';
grant select, update, delete, insert on apiguard_client_actions to 'apiguard'@'192.168.1.%';
grant select, update, delete, insert on apiguard_client_counting_rules to 'apiguard'@'192.168.1.%';
grant select, update, delete, insert on apiguard_client_stats to 'apiguard'@'192.168.1.%';
grant select, update, delete, insert on apiguard_delay_log to 'apiguard'@'192.168.1.%';
grant select on user_session to 'apiguard'@'192.168.1.%';
grant select on user to 'apiguard'@'192.168.1.%';
grant select, update, delete, insert on api_user_ban to 'apiguard'@'192.168.1.%';
grant select, update, delete, insert on api_ip_ban to 'apiguard'@'192.168.1.%';

*/

const (
	DfltUsersTableName = "user"
)

type Conf struct {
	Name                   string `json:"name"`
	Host                   string `json:"host"`
	User                   string `json:"user"`
	Password               string `json:"password"`
	OverrideUsersTableName string `json:"overrideUsersTableName"`
	AnonymousUserID        int    `json:"anonymousUserId"`
}

func (conf *Conf) Validate(context string) error {
	if conf.Name == "" && conf.Host == "" && conf.User == "" && conf.Password == "" {
		log.Warn().Msgf("CNC database not configured - related functions will be disabled")
		return nil

	} else if conf.Name == "" {
		return fmt.Errorf("%s.name is missing/empty", context)

	} else if conf.Host == "" {
		return fmt.Errorf("%s.host is missing/empty", context)

	} else if conf.User == "" {
		return fmt.Errorf("%s.user is missing/empty", context)

	} else if conf.Password == "" {
		return fmt.Errorf("%s.password is missing/empty", context)
	}
	return nil
}

func OpenDB(conf *Conf) (*sql.DB, error) {
	mconf := mysql.NewConfig()
	mconf.Net = "tcp"
	mconf.Addr = conf.Host
	mconf.User = conf.User
	mconf.Passwd = conf.Password
	mconf.DBName = conf.Name
	mconf.ParseTime = true
	mconf.Loc = time.Local
	mconf.Params = map[string]string{"autocommit": "true"}
	db, err := sql.Open("mysql", mconf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return db, nil
}
