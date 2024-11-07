// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package session

type NoneSessionValue struct {
}

func (nsv NoneSessionValue) String() string {
	return ""
}

func (nsv NoneSessionValue) IsZero() bool {
	return true
}

func (nsv NoneSessionValue) SrchSelector() string {
	return ""
}

func (nsv NoneSessionValue) CompareWithStoredVal(h string) (bool, error) {
	return false, nil
}

func (nsv NoneSessionValue) UpdatedFrom(v string) HTTPSession {
	return NoneSessionValue{}
}
