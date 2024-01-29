package session

import "strings"

type CNCSessionValue struct {
	Selector  string
	Validator string
}

func (cv CNCSessionValue) UpdateFrom(v string) {
	tmp := strings.SplitN(v, "-", 2)
	if len(tmp) > 1 {
		cv.Selector = tmp[0]
		cv.Validator = tmp[1]

	} else {
		cv.Selector = ""
		cv.Validator = ""
	}
}

func (cv CNCSessionValue) IsZero() bool {
	return cv.Selector == "" && cv.Validator == ""
}
