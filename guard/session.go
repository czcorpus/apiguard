// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package guard

import (
	"apiguard/session"
	"fmt"
)

func CreateSessionValFactory(st session.SessionType) func() session.HTTPSession {
	switch st {
	case session.SessionTypeCNC:
		return func() session.HTTPSession { return session.CNCSessionValue{} }
	case session.SessionTypeSimple:
		return func() session.HTTPSession { return session.SimpleSessionValue{} }
	case session.SessionTypeNone:
		return func() session.HTTPSession { return session.NoneSessionValue{} }
	}
	panic(fmt.Errorf("unsupported session value type: `%s`", st))
}
