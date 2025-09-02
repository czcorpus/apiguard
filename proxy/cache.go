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

	"github.com/rs/zerolog/log"
)

var ErrCacheMiss = errors.New("cache miss")

func GenerateCacheId(req *http.Request, opts *CacheEntryOptions) []byte {
	h := sha1.New()
	h.Write([]byte(req.URL.Path))
	h.Write([]byte(req.URL.Query().Encode()))
	if len(opts.RespectCookies) > 0 {
		hashCookies := make([]string, 0)
		for _, respectCookie := range opts.RespectCookies {
			respCookie, err := req.Cookie(respectCookie)
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
	if len(opts.RequestBody) > 0 {
		h.Write(opts.RequestBody)
	}
	return h.Sum(nil)
}

// ShouldReadFromCache tests if the provided request and options match
// caching conditions for reading.
func ShouldReadFromCache(req *http.Request, opts *CacheEntryOptions) bool {
	return (req.Method == http.MethodGet || opts.CacheablePOST) &&
		req.Header.Get("Cache-Control") != "no-cache"
}

// ShouldWriteToCache tests if the provided user request and response properties
// make the response a valid candidate for caching.
func ShouldWriteToCache(req *http.Request, value CacheEntry, opts *CacheEntryOptions) bool {
	ans := (value.Status == http.StatusOK || value.Status == http.StatusCreated) &&
		(req.Method == http.MethodGet || opts.CacheablePOST) &&
		req.Header.Get("Cache-Control") != "no-cache"
	log.Debug().
		Str("url", req.URL.String()).
		Bool("cacheable", ans).
		Str("httpCacheControl", req.Header.Get("Cache-Control")).
		Msg("testing cacheability")
	return ans
}

type CacheConf struct {
	FileRootPath string `json:"fileRootPath"`
	RedisAddr    string `json:"redisAddr"`
	RedisDB      int    `json:"redisDB"`
	TTLSecs      int    `json:"ttlSecs"`
}
