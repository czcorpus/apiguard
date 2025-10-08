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
