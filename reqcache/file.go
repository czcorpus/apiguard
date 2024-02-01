// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"apiguard/proxy"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
)

type FileReqCache struct {
	conf *Conf
}

func (frc *FileReqCache) createItemPath(req *http.Request, resp proxy.BackendResponse, respectCookies []string) string {
	cacheID := generateCacheId(req, resp, respectCookies)
	bs := fmt.Sprintf("%x.gob", cacheID)
	return path.Join(frc.conf.FileRootPath, bs[0:1], bs)
}

func (rc *FileReqCache) Get(req *http.Request, respectCookies []string) (proxy.BackendResponse, error) {
	filePath := rc.createItemPath(req, nil, respectCookies)
	isFile, err := fs.IsFile(filePath)
	if err != nil {
		return nil, err
	}
	if !isFile {
		return nil, ErrCacheMiss
	}
	mtime, err := fs.GetFileMtime(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain file mtime: %w", err)
	}
	if time.Since(mtime) > time.Duration(rc.conf.TTLSecs)*time.Second {
		err := fs.DeleteFile(filePath)
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
	var ans proxy.BackendResponse
	err = dec.Decode(&ans)
	if err == nil {
		ans.MarkCached()
	}
	return ans, err
}

func (frc *FileReqCache) Set(req *http.Request, resp proxy.BackendResponse, respectCookies []string) error {
	if resp.GetStatusCode() == http.StatusOK && resp.GetError() == nil &&
		req.Method == http.MethodGet && req.Header.Get("Cache-Control") != "no-cache" {
		targetPath := frc.createItemPath(req, resp, respectCookies)
		os.MkdirAll(path.Dir(targetPath), os.ModePerm)
		fw, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		enc := gob.NewEncoder(fw)
		return enc.Encode(&resp)
	}
	return nil
}

func NewFileReqCache(conf *Conf) *FileReqCache {
	return &FileReqCache{
		conf: conf,
	}
}
