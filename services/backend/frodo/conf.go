// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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

package frodo

import (
	"apiguard/guard"
	"apiguard/guard/token"
	"apiguard/services/cnc"
	"fmt"
)

type Conf struct {
	cnc.ProxyConf
	GuardType       guard.GuardType   `json:"guardType"`
	TokenHeaderName string            `json:"tokenHeaderName"`
	Tokens          []token.TokenConf `json:"tokens"`
}

func (c *Conf) Validate(context string) error {
	if err := c.ProxyConf.Validate(context); err != nil {
		return err
	}
	if c.GuardType == guard.GuardTypeToken && len(c.Tokens) == 0 {
		return fmt.Errorf("no tokens defined for token guard - the service won't be accessible")
	}
	return nil
}
