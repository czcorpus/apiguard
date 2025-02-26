// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"crypto/sha1"
	"errors"
	"net/http"
	"sort"
	"strings"
)

var ErrCacheMiss = errors.New("cache miss")

func GenerateCacheId(req *http.Request, resp BackendResponse, respectCookies []string) []byte {
	h := sha1.New()
	h.Write([]byte(req.URL.Path))
	h.Write([]byte(req.URL.Query().Encode()))
	if respectCookies != nil {
		hashCookies := make([]string, 0)
		respParams := http.Request{}
		if resp != nil {
			respParams.Header = resp.GetHeaders()
		}
		for _, respectCookie := range respectCookies {
			respCookie, err := respParams.Cookie(respectCookie)
			if err == nil {
				hashCookies = append(hashCookies, respCookie.Name+"="+respCookie.Value)
				continue
			}
			reqCookie, err := req.Cookie(respectCookie)
			if err == nil {
				hashCookies = append(hashCookies, reqCookie.Name+"="+reqCookie.Value)
			}
		}
		sort.Strings(hashCookies)
		h.Write([]byte(strings.Join(hashCookies, ";")))
	}
	return h.Sum(nil)
}

type CacheConf struct {
	FileRootPath string `json:"fileRootPath"`
	RedisAddr    string `json:"redisAddr"`
	RedisDB      int    `json:"redisDB"`
	TTLSecs      int    `json:"ttlSecs"`
}
