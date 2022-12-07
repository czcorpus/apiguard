// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"apiguard/fsops"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

type Conf struct {
	RootPath string `json:"rootPath"`
	TTLSecs  int    `json:"ttlSecs"`
}

type ReqCache struct {
	conf *Conf
}

type CacheData struct {
	Header *http.Header `json:"header"`
	Body   string       `json:"body"`
}

func (rc *ReqCache) createItemPath(url string) string {
	h := sha1.New()
	h.Write([]byte(url))
	bs := fmt.Sprintf("%x.html", h.Sum(nil))
	return path.Join(rc.conf.RootPath, bs[0:1], bs)
}

func (rc *ReqCache) Get(url string) (string, *http.Header, error) {
	filePath := rc.createItemPath(url)
	if !fsops.IsFile(filePath) ||
		time.Since(fsops.GetFileMtime(filePath)) > time.Duration(rc.conf.TTLSecs)*time.Second {
		return "", nil, ErrCacheMiss
	}
	newTime := time.Now()
	err := os.Chtimes(filePath, newTime, newTime)
	if err != nil {
		return "", nil, err
	}
	rawData, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil, err
	}
	cacheData := CacheData{}
	err = json.Unmarshal(rawData, &cacheData)
	if err != nil {
		return "", nil, err
	}
	return cacheData.Body, cacheData.Header, err
}

func (rc *ReqCache) Set(url, body string, header *http.Header, req *http.Request) error {
	if req.Method == http.MethodGet && req.Header.Get("Cache-Control") != "no-cache" {
		targetPath := rc.createItemPath(url)
		os.MkdirAll(path.Dir(targetPath), os.ModePerm)
		rawData, err := json.Marshal(CacheData{
			Header: header,
			Body:   body,
		})
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, rawData, 0644)
	}
	return nil
}

func NewReqCache(conf *Conf) *ReqCache {
	return &ReqCache{conf: conf}
}
