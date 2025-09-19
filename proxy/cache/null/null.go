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

package null

import (
	"net/http"

	"github.com/czcorpus/apiguard-common/cache"
	"github.com/czcorpus/apiguard/proxy"
)

type NullCache struct{}

func (rc *NullCache) Get(req *http.Request, opts ...func(*cache.CacheEntryOptions)) (cache.CacheEntry, error) {
	return cache.CacheEntry{}, proxy.ErrCacheMiss
}

func (rc *NullCache) Set(req *http.Request, value cache.CacheEntry, opts ...func(*cache.CacheEntryOptions)) error {
	return nil
}

func New() *NullCache {
	return &NullCache{}
}
