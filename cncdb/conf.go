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
) ENGINE=InnoDB;

CREATE TABLE api_user_ban (
	id INTEGER NOT NULL auto_increment,
	report_id VARCHAR(36), -- report ID is stored only by a running service
	user_id INTEGER NOT NULL,
	start_dt DATETIME NOT NULL,
	end_dt DATETIME NOT NULL,
	active TINYINT NOT NULL DEFAULT 1, -- so we can disable it any time and also to archive records
	PRIMARY KEY (id),
	FOREIGN KEY (user_id) REFERENCES kontext_user(id)
) ENGINE=InnoDB;

CREATE TABLE apiguard_session_conf (
	session_id int(11) NOT NULL,
	data TEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (session_id),
	FOREIGN KEY (session_id) REFERENCES user_session(id)
) ENGINE=InnoDB;

CREATE TABLE api_user_allowlist (
	id int(11) NOT NULL AUTO_INCREMENT,
	service_name varchar(25) NOT NULL,
	user_id INTEGER NOT NULL,
	PRIMARY KEY (id),
	UNIQUE KEY uc_user_app (service_name, user_id),
	FOREIGN KEY (user_id) REFERENCES kontext_user(id)
) ENGINE=InnoDB;

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
grant select, update, delete, insert on apiguard_session_conf to 'apiguard'@'192.168.1.%';

*/

const (
	DfltUsersTableName   = "user"
	DfltUsernameColName  = "user"
	DfltFirstnameColName = "firstName"
	DfltLastnameColName  = "surname"
)

type UserTableProps struct {
	UserTableName    string
	UsernameColName  string
	FirstnameColName string
	LastnameColName  string
}

type Conf struct {
	Name                     string        `json:"name"`
	Host                     string        `json:"host"`
	User                     string        `json:"user"`
	Password                 string        `json:"password"`
	OverrideUserTableName    string        `json:"overrideUserTableName"`
	OverrideUsernameColName  string        `json:"overrideUsernameColName"`
	OverrideFirstnameColName string        `json:"overrideFirstnameColName"`
	OverrideLastnameColName  string        `json:"overrideLastnameColName"`
	AnonymousUserID          common.UserID `json:"anonymousUserId"`
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

func (conf *Conf) ApplyOverrides() UserTableProps {
	var ans UserTableProps
	ans.UserTableName = DfltUsersTableName
	if conf.OverrideUserTableName != "" {
		ans.UserTableName = conf.OverrideUserTableName
		log.Warn().Msgf("overriding users table name to '%s'", ans.UserTableName)
	}
	ans.UsernameColName = DfltUsernameColName
	if conf.OverrideUsernameColName != "" {
		ans.UsernameColName = conf.OverrideUsernameColName
		log.Warn().Msgf("overriding username column name in user table to '%s'", ans.UsernameColName)
	}
	ans.FirstnameColName = DfltFirstnameColName
	if conf.OverrideFirstnameColName != "" {
		ans.FirstnameColName = conf.OverrideFirstnameColName
		log.Warn().Msgf("overriding 'first name' column name in user table to '%s'", ans.FirstnameColName)
	}
	ans.LastnameColName = DfltLastnameColName
	if conf.OverrideLastnameColName != "" {
		ans.LastnameColName = conf.OverrideLastnameColName
		log.Warn().Msgf("overriding 'last name' column name in user table to '%s'", ans.LastnameColName)
	}
	return ans
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
