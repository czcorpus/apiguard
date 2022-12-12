// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import "errors"

var ErrCacheMiss = errors.New("cache miss")

type Conf struct {
	FileRootPath string `json:"fileRootPath"`
	RedisAddr    string `json:"redisAddr"`
	RedisDB      int    `json:"redisDB"`
	TTLSecs      int    `json:"ttlSecs"`
}
