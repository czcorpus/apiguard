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

package cja

import "fmt"

type Conf struct {
	BaseURL         string `json:"baseURL"`
	ClientUserAgent string `json:"clientUserAgent"`
}

func (c *Conf) Validate(context string) error {
	if c.BaseURL == "" {
		return fmt.Errorf("%s.baseURL is missing/empty", context)
	}
	return nil
}
