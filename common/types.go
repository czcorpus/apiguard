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

func Dur2Hms(dur time.Duration) string {
	numSec := int(dur.Seconds())
	if numSec < 0 {
		panic("Sec2hms requires non-negative numbers")
	}
	hours := numSec / 3600
	rest := numSec % 3600
	mins := rest / 60
	rest = rest % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, rest)

}
