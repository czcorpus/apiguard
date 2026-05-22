// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Department of Linguistics,
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

package globctx

import (
	"sync"

	"github.com/czcorpus/klogproc-core/storage"
)

type ESDispatcher struct {
	ch     chan *storage.OnTheFlyOutputRecord
	wg     sync.WaitGroup
	once   sync.Once
	closed chan struct{}
}

func NewESDispatcher(bufferSize int) *ESDispatcher {
	return &ESDispatcher{
		ch:     make(chan *storage.OnTheFlyOutputRecord, bufferSize),
		closed: make(chan struct{}),
	}
}

func (d *ESDispatcher) Send(e *storage.OnTheFlyOutputRecord) bool {
	d.wg.Add(1)
	defer d.wg.Done()

	select {
	case <-d.closed:
		return false // shutdown in progress, drop or handle
	case d.ch <- e:
		return true
	}
}

func (d *ESDispatcher) Shutdown() {
	d.once.Do(func() {
		close(d.closed) // signal: no new sends accepted
		d.wg.Wait()     // wait for in-flight Send() calls to finish
		close(d.ch)     // now safe to close
	})
}

func (d *ESDispatcher) Ch() <-chan *storage.OnTheFlyOutputRecord {
	return d.ch
}
