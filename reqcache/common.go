// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"apiguard/services"
	"crypto/sha1"
	"errors"
	"net/http"
	"sort"
	"strings"
)

var ErrCacheMiss = errors.New("cache miss")

func generateCacheId(req *http.Request, resp services.BackendResponse, respectCookies []string) []byte {
	h := sha1.New()
	h.Write([]byte(req.URL.Path))
	h.Write([]byte(req.URL.Query().Encode()))
	if respectCookies != nil {
		hashCookies := make([]string, 0)
		var respCookies []string
		if resp == nil {
			respCookies = make([]string, 0)
		} else {
			respCookies = strings.Split(resp.GetHeaders().Get("Cookies"), ";")
		}
		for _, respectCookie := range respectCookies {
			filledFromResponse := false
			for _, respCookie := range respCookies {
				kv := strings.Split(strings.TrimSpace(respCookie), "=")
				if kv[0] == respectCookie {
					if len(kv) > 1 {
						hashCookies = append(hashCookies, kv[0]+"="+kv[1])
					} else {
						hashCookies = append(hashCookies, kv[0]+"=")
					}
					filledFromResponse = true
					break
				}
			}
			if !filledFromResponse {
				reqCookie, err := req.Cookie(respectCookie)
				if err == nil {
					hashCookies = append(hashCookies, reqCookie.Name+"="+reqCookie.Value)
				}
			}
		}
		sort.Strings(hashCookies)
		h.Write([]byte(strings.Join(hashCookies, ";")))
	}
	return h.Sum(nil)
}

type Conf struct {
	FileRootPath string `json:"fileRootPath"`
	RedisAddr    string `json:"redisAddr"`
	RedisDB      int    `json:"redisDB"`
	TTLSecs      int    `json:"ttlSecs"`
}
