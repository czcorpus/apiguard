// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

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
