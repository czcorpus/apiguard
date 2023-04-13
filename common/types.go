// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package common

import (
	"math"
	"strconv"
	"time"
)

type UserID int

type CheckInterval time.Duration

func (ci CheckInterval) ToSeconds() int {
	return int(math.RoundToEven(time.Duration(ci).Seconds()))
}

func Str2UserID(v string) (UserID, error) {
	tmp, err := strconv.Atoi(v)
	if err != nil {
		return -1, err
	}
	return UserID(tmp), nil
}
