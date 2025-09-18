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

package wagstream

import (
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/google/uuid"
)

const (
	maxRecAge = 5 * time.Minute
)

type rec struct {
	Data    *StreamRequestJSON
	Created time.Time
}

type streams struct {
	data *collections.ConcurrentMap[string, rec]
}

func (s *streams) Add(data *StreamRequestJSON) string {
	r := rec{
		Data:    data,
		Created: time.Now(),
	}
	id := uuid.New().String()
	s.data.Set(id, r)
	return id
}

func (s *streams) Get(id string) *StreamRequestJSON {
	v, ok := s.data.GetWithTest(id)
	if !ok {
		return nil
	}
	return v.Data
}

func (s *streams) cleanup() {
	s.data = s.data.Filter(
		func(k string, v rec) bool {
			return time.Since(s.data.Get(k).Created) <= maxRecAge
		},
	)
}

func newStreams() *streams {
	return &streams{
		data: collections.NewConcurrentMap[string, rec](),
	}
}
