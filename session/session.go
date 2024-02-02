package session

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

// CNCSessionValue represents a parsed CNC session where
// two values are used - one for fast session row selection
// and the other one (Validator) for secure value comparison
type CNCSessionValue struct {
	Selector  string
	Validator string
}

// String returns a stardard value representation
// as stored in CNC session database.
func (cv CNCSessionValue) String() string {
	return fmt.Sprintf("%s-%s", cv.Selector, cv.Validator)
}

// UpdateFrom sets `Selector` and `Validator` using parsed
// values from a raw session value `v`
func (cv *CNCSessionValue) UpdateFrom(v string) {
	tmp := strings.SplitN(v, "-", 2)
	if len(tmp) > 1 {
		cv.Selector = tmp[0]
		cv.Validator = tmp[1]

	} else {
		cv.Selector = ""
		cv.Validator = ""
	}
}

// IsZero tests for empty value (both `Selector` and `Validator` must be empty)
func (cv CNCSessionValue) IsZero() bool {
	return cv.Selector == "" && cv.Validator == ""
}

// CompareWithHash compares `Validator` with provided hash `h`.
// (the `Validator` is hashed with sha256 before comparing and
// the comparing function works in constant time no matter how
// long common prefix the compared values have (but that works
// only for values of the same size))
func (cv CNCSessionValue) CompareWithHash(h string) (bool, error) {
	hasher := sha256.New()
	if _, err := hasher.Write([]byte(cv.Validator)); err != nil {
		return false, err
	}
	hashedValidator := fmt.Sprintf("%x", hasher.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(hashedValidator), []byte(h)) == 1 {
		return true, nil
	}
	return false, nil
}
