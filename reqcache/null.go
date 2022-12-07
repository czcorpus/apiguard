// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import "net/http"

type NullCache struct{}

func (rc *NullCache) Get(url string) (string, *http.Header, error) {
	return "", nil, ErrCacheMiss
}

func (rc *NullCache) Set(url, body string, req *http.Request) error {
	return nil
}

func NewNullCache() *NullCache {
	return &NullCache{}
}
