// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/czcorpus/apiguard-common/cache"
	"github.com/czcorpus/apiguard/proxy"

	"github.com/czcorpus/cnc-gokit/fs"
)

type File struct {
	conf *proxy.CacheConf
}

func (frc *File) createItemPath(req *http.Request, opts *cache.CacheEntryOptions) string {
	cacheID := proxy.GenerateCacheId(req, opts)
	bs := fmt.Sprintf("%x.gob", cacheID)
	return path.Join(frc.conf.FileRootPath, bs[0:1], bs)
}

func (rc *File) Get(req *http.Request, opts ...func(*cache.CacheEntryOptions)) (cache.CacheEntry, error) {
	optsFin := new(cache.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if !proxy.ShouldReadFromCache(req, optsFin) {
		return cache.CacheEntry{}, proxy.ErrCacheMiss
	}
	filePath := rc.createItemPath(req, optsFin)
	isFile, err := fs.IsFile(filePath)
	if err != nil {
		return cache.CacheEntry{}, err
	}
	if !isFile {
		return cache.CacheEntry{}, proxy.ErrCacheMiss
	}
	mtime, err := fs.GetFileMtime(filePath)
	if err != nil {
		return cache.CacheEntry{}, fmt.Errorf("failed to obtain file mtime: %w", err)
	}
	if time.Since(mtime) > time.Duration(rc.conf.TTLSecs)*time.Second {
		err := fs.DeleteFile(filePath)
		if err != nil {
			return cache.CacheEntry{}, err
		}
		return cache.CacheEntry{}, proxy.ErrCacheMiss
	}
	newTime := time.Now()
	err = os.Chtimes(filePath, newTime, newTime)
	if err != nil {
		return cache.CacheEntry{}, err
	}
	fr, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return cache.CacheEntry{}, err
	}
	decoder := gob.NewDecoder(fr)
	var ans cache.CacheEntry
	err = decoder.Decode(&ans)
	if err == nil {
		return ans, nil
	}
	return ans, fmt.Errorf("proxy cache access error: %w", err)
}

func (frc *File) Set(req *http.Request, value cache.CacheEntry, opts ...func(*cache.CacheEntryOptions)) error {
	optsFin := new(cache.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if proxy.ShouldWriteToCache(req, value, optsFin) {
		targetPath := frc.createItemPath(req, optsFin)
		os.MkdirAll(path.Dir(targetPath), os.ModePerm)
		fw, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		enc := gob.NewEncoder(fw)
		return enc.Encode(&value)
	}
	return nil
}

func New(conf *proxy.CacheConf) *File {
	return &File{
		conf: conf,
	}
}
