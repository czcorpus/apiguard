// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// -------------------------

// UserID is just a specialized int with some
// convenience methods
type UserID int

func (u UserID) IsValid() bool {
	return u > InvalidUserID
}

func (u UserID) String() string {
	return fmt.Sprintf("%d", u)
}

//

type AnonymousUsers []UserID

func (au AnonymousUsers) IsAnonymous(uid UserID) bool {
	for _, v := range au {
		if v == uid {
			return true
		}
	}
	return false
}

// ---------------- CheckInterval ------------------

// CheckInterval specifies an interval to which we evaluate
// a specific limit. E.g. 'max X requests per 1 hour'.
// Apiguard allows multiple intervals with their limits
// e.g. for hourly and daily limits.
type CheckInterval time.Duration

func (ci CheckInterval) ToSeconds() int {
	return int(math.RoundToEven(time.Duration(ci).Seconds()))
}

func (ci CheckInterval) String() string {
	return time.Duration(ci).String()
}

// -----------------------------

// Str2UserID parses a string with encoded numeric
// user ID. In case of an error, it returns InvalidUserID
// along with the error.
func Str2UserID(v string) (UserID, error) {
	tmp, err := strconv.Atoi(v)
	if err != nil {
		return InvalidUserID, err
	}
	return UserID(tmp), nil
}

// ClientID is a general identifier of an end user client.
// We always prefer UserID if we can extract one, but
// the type also allows for unknown users
// identifiable (roughly) via IP.
type ClientID struct {
	IP string
	ID UserID
}

// GetKey is used in situations where we need
// to address some user-specific information within
// hash maps etc.
func (c *ClientID) GetKey() string {
	return fmt.Sprintf("%s:%d", c.IP, c.ID)
}
