// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

func isFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	finfo, err := f.Stat()
	if err != nil {
		return false
	}
	return finfo.Mode().IsRegular()
}

func getFileMtime(filePath string) time.Time {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}
	}
	finfo, err := f.Stat()
	if err == nil {
		return time.Unix(finfo.ModTime().Unix(), 0)

	}
	return time.Time{}
}

type Conf struct {
	RootPath string `json:"rootPath"`
	TTLSecs  int    `json:"ttlSecs"`
}

type ReqCache struct {
	conf *Conf
}

func (rc *ReqCache) createItemPath(url string) string {
	h := sha1.New()
	h.Write([]byte(url))
	bs := fmt.Sprintf("%x.html", h.Sum(nil))
	return path.Join(rc.conf.RootPath, bs[0:1], bs)
}

func (rc *ReqCache) Get(url string) (string, error) {
	filePath := rc.createItemPath(url)
	if !isFile(filePath) ||
		time.Since(getFileMtime(filePath)) > time.Duration(rc.conf.TTLSecs)*time.Second {
		return "", ErrCacheMiss
	}
	newTime := time.Now()
	err := os.Chtimes(filePath, newTime, newTime)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(filePath)
	return string(body), err
}

func (rc *ReqCache) Set(url, body string) error {
	targetPath := rc.createItemPath(url)
	os.MkdirAll(path.Dir(targetPath), os.ModePerm)
	return os.WriteFile(targetPath, []byte(body), 0644)
}

func NewReqCache(conf *Conf) *ReqCache {
	return &ReqCache{conf: conf}
}
