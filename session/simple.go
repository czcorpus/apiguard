// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package session

type SimpleSessionValue struct {
	Value string
}

func (cv SimpleSessionValue) SrchSelector() string {
	return cv.Value[:12]
}

func (cv SimpleSessionValue) String() string {
	return cv.Value
}

func (cv SimpleSessionValue) UpdatedFrom(v string) HTTPSession {
	return SimpleSessionValue{Value: v}
}

func (cv SimpleSessionValue) IsZero() bool {
	return cv.Value == ""
}

func (cv SimpleSessionValue) CompareWithStoredVal(h string) (bool, error) {
	return cv.Value == h, nil
}
