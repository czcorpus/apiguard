// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reqcache

import (
	"apiguard/proxy"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisReqCache struct {
	conf         *Conf
	redisClient  *redis.Client
	redisContext context.Context
}

func (rrc *RedisReqCache) createCacheID(req *http.Request, resp proxy.BackendResponse, respectCookies []string) string {
	cacheID := generateCacheId(req, resp, respectCookies)
	return fmt.Sprintf("apiguard:cache:%x", cacheID)
}

func (rrc *RedisReqCache) Get(req *http.Request, respectCookies []string) (proxy.BackendResponse, error) {
	cacheID := rrc.createCacheID(req, nil, respectCookies)
	val, err := rrc.redisClient.Get(rrc.redisContext, cacheID).Result()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	} else if err != nil {
		return nil, err
	}
	_, err = rrc.redisClient.Expire(rrc.redisContext, cacheID, time.Duration(rrc.conf.TTLSecs)*time.Second).Result()
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader([]byte(val))
	decoder := gob.NewDecoder(reader)
	var ans proxy.BackendResponse
	err = decoder.Decode(&ans)
	if err == nil {
		ans.MarkCached()
	}
	return ans, err
}

func (rrc *RedisReqCache) Set(req *http.Request, resp proxy.BackendResponse, respectCookies []string) error {
	if resp.GetStatusCode() == http.StatusOK && resp.GetError() == nil &&
		req.Method == http.MethodGet && req.Header.Get("Cache-Control") != "no-cache" {
		cacheID := rrc.createCacheID(req, resp, respectCookies)
		var buffer bytes.Buffer
		encoder := gob.NewEncoder(&buffer)
		err := encoder.Encode(&resp)
		if err != nil {
			return err
		}
		_, err = rrc.redisClient.Set(rrc.redisContext, cacheID, buffer.String(), time.Duration(rrc.conf.TTLSecs)*time.Second).Result()
		if err != nil {
			return err
		}
	}
	return nil
}

func NewRedisReqCache(conf *Conf) *RedisReqCache {
	return &RedisReqCache{
		conf: conf,
		redisClient: redis.NewClient(&redis.Options{
			Addr: conf.RedisAddr,
			DB:   conf.RedisDB,
		}),
		redisContext: context.Background(),
	}
}
