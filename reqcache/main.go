// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"apiguard/fsops"
	"apiguard/services"
	"crypto/sha1"
	"encoding/gob"
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

func (rc *ReqCache) createItemPath(req *http.Request) string {
	h := sha1.New()
	h.Write([]byte(req.URL.Path))
	h.Write([]byte(req.URL.Query().Encode()))
	h.Write([]byte(req.Header.Get("Cookie")))
	bs := fmt.Sprintf("%x.gob", h.Sum(nil))
	return path.Join(rc.conf.RootPath, bs[0:1], bs)
}

func (rc *ReqCache) Get(req *http.Request) (services.BackendResponse, error) {
	filePath := rc.createItemPath(req)
	isFile, err := fsops.IsFile(filePath)
	if err != nil {
		return nil, err
	}
	if !isFile {
		return nil, ErrCacheMiss
	}
	if time.Since(fsops.GetFileMtime(filePath)) > time.Duration(rc.conf.TTLSecs)*time.Second {
		err := fsops.DeleteFile(filePath)
		if err != nil {
			return nil, err
		}
		return nil, ErrCacheMiss
	}
	newTime := time.Now()
	err = os.Chtimes(filePath, newTime, newTime)
	if err != nil {
		return nil, err
	}
	fr, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(fr)
	var ans services.BackendResponse
	err = dec.Decode(&ans)
	return ans, err
}

func (rc *ReqCache) Set(req *http.Request, resp services.BackendResponse) error {
	if resp.GetStatusCode() == http.StatusOK && resp.GetError() == nil &&
		req.Method == http.MethodGet && req.Header.Get("Cache-Control") != "no-cache" {
		targetPath := rc.createItemPath(req)
		os.MkdirAll(path.Dir(targetPath), os.ModePerm)
		fw, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		resp.MarkCached()
		enc := gob.NewEncoder(fw)
		return enc.Encode(&resp)
	}
	return nil
}

func NewReqCache(conf *Conf) *ReqCache {
	return &ReqCache{
		conf: conf,
	}
}
