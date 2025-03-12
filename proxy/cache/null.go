// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cache

import (
	"apiguard/proxy"
	"net/http"
)

type NullCache struct{}

func (rc *NullCache) Get(req *http.Request, opts ...func(*proxy.CacheEntryOptions)) (proxy.BackendResponse, error) {
	return nil, proxy.ErrCacheMiss
}

func (rc *NullCache) Set(req *http.Request, resp proxy.BackendResponse, opts ...func(*proxy.CacheEntryOptions)) error {
	return nil
}

func NewNullCache() *NullCache {
	return &NullCache{}
}
