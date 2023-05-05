// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package common

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	// InvalidUserID represents an unknown/undefined user.
	// Please note that this is different than "anonymous user"
	// which is typically an existing database ID (available via
	// APIGuard configuration).
	InvalidUserID UserID = -1
)

type UserID int

func (u UserID) IsValid() bool {
	return u > InvalidUserID
}

func (u UserID) String() string {
	return fmt.Sprintf("%d", u)
}

type CheckInterval time.Duration

func (ci CheckInterval) ToSeconds() int {
	return int(math.RoundToEven(time.Duration(ci).Seconds()))
}

func Str2UserID(v string) (UserID, error) {
	tmp, err := strconv.Atoi(v)
	if err != nil {
		return InvalidUserID, err
	}
	return UserID(tmp), nil
}
