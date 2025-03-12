// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cache

import (
	"apiguard/proxy"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

const (
	defaultRedisPort = 6379
)

type Redis struct {
	conf         *proxy.CacheConf
	redisClient  *redis.Client
	redisContext context.Context
}

func (rrc *Redis) createCacheID(req *http.Request, resp proxy.BackendResponse, opts *proxy.CacheEntryOptions) string {
	cacheID := proxy.GenerateCacheId(req, resp, opts)
	return fmt.Sprintf("apiguard:cache:%x", cacheID)
}

func (rrc *Redis) Get(req *http.Request, opts ...func(*proxy.CacheEntryOptions)) (proxy.BackendResponse, error) {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	cacheID := rrc.createCacheID(req, nil, optsFin)
	val, err := rrc.redisClient.Get(rrc.redisContext, cacheID).Result()
	if err == redis.Nil {
		return nil, proxy.ErrCacheMiss
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

func (rrc *Redis) Set(req *http.Request, resp proxy.BackendResponse, opts ...func(*proxy.CacheEntryOptions)) error {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if proxy.IsCacheableProxying(req, resp, optsFin) {
		cacheID := rrc.createCacheID(req, resp, optsFin)
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

func NewRedisCache(conf *proxy.CacheConf) *Redis {
	addr := conf.RedisAddr
	addrElms := strings.Split(":", addr)
	if len(addrElms) == 1 {
		addr = fmt.Sprintf("%s:%d", addr, defaultRedisPort)
		log.Warn().Msgf("Caching: Redis port not specified, using %d", defaultRedisPort)
	}
	return &Redis{
		conf: conf,
		redisClient: redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   conf.RedisDB,
		}),
		redisContext: context.Background(),
	}
}
