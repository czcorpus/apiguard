// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cache

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

type File struct {
	conf *proxy.CacheConf
}

func (frc *File) createItemPath(req *http.Request, resp proxy.BackendResponse, opts *proxy.CacheEntryOptions) string {
	cacheID := proxy.GenerateCacheId(req, resp, opts)
	bs := fmt.Sprintf("%x.gob", cacheID)
	return path.Join(frc.conf.FileRootPath, bs[0:1], bs)
}

func (rc *File) Get(req *http.Request, opts ...func(*proxy.CacheEntryOptions)) (proxy.BackendResponse, error) {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	filePath := rc.createItemPath(req, nil, optsFin)
	isFile, err := fs.IsFile(filePath)
	if err != nil {
		return nil, err
	}
	if !isFile {
		return nil, proxy.ErrCacheMiss
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
		return nil, proxy.ErrCacheMiss
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

func (frc *File) Set(req *http.Request, resp proxy.BackendResponse, opts ...func(*proxy.CacheEntryOptions)) error {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if proxy.IsCacheableProxying(req, resp, optsFin) {
		targetPath := frc.createItemPath(req, resp, optsFin)
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

func NewFileCache(conf *proxy.CacheConf) *File {
	return &File{
		conf: conf,
	}
}
