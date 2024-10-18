// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package session

import "fmt"

type SessionType string

func (st SessionType) Validate() error {
	if st != SessionTypeCNC && st != SessionTypeSimple {
		return fmt.Errorf("invalid session type: `%s`", st)
	}
	return nil
}

const (
	SessionTypeCNC    SessionType = "cnc"
	SessionTypeSimple SessionType = "simple"
)

type HTTPSession interface {
	String() string
	IsZero() bool
	SrchSelector() string
	CompareWithStoredVal(h string) (bool, error)
	UpdatedFrom(v string) HTTPSession
}
