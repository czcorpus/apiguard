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
