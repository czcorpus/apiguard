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
	defaultRedisPort     = 6379
	writeChannelCapacity = 100
)

type writeQueueItem struct {
	key   string
	value []byte
}

type Redis struct {
	ctx         context.Context
	conf        *proxy.CacheConf
	redisClient *redis.Client
	writeQueue  chan writeQueueItem
}

func (rrc *Redis) createCacheID(req *http.Request, opts *proxy.CacheEntryOptions) string {
	cacheID := proxy.GenerateCacheId(req, opts)
	return fmt.Sprintf("apiguard:cache:%x", cacheID)
}

func (rrc *Redis) Get(req *http.Request, opts ...func(*proxy.CacheEntryOptions)) (proxy.CacheEntry, error) {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if !proxy.ShouldReadFromCache(req, optsFin) {
		return proxy.CacheEntry{}, proxy.ErrCacheMiss
	}
	cacheID := rrc.createCacheID(req, optsFin)
	val, err := rrc.redisClient.Get(rrc.ctx, cacheID).Result()
	if err == redis.Nil {
		return proxy.CacheEntry{}, proxy.ErrCacheMiss

	} else if err != nil {
		return proxy.CacheEntry{}, fmt.Errorf("proxy cache access error: %w", err)
	}
	_, err = rrc.redisClient.Expire(rrc.ctx, cacheID, time.Duration(rrc.conf.TTLSecs)*time.Second).Result()
	if err != nil {
		return proxy.CacheEntry{}, fmt.Errorf("proxy cache access error: %w", err)
	}
	reader := bytes.NewReader([]byte(val))
	decoder := gob.NewDecoder(reader)
	var ans proxy.CacheEntry
	err = decoder.Decode(&ans)
	if err == nil {
		return ans, nil
	}
	return ans, fmt.Errorf("proxy cache access error: %w", err)
}

func (rrc *Redis) goRunWriter() {
	go func() {
		for {
			select {
			case <-rrc.ctx.Done():
				log.Warn().Msg("closing Redis cache writing queue")
				return
			case wrItem := <-rrc.writeQueue:
				_, err := rrc.redisClient.Set(rrc.ctx, wrItem.key, wrItem.value, time.Duration(rrc.conf.TTLSecs)*time.Second).Result()
				if err != nil {
					log.Error().
						Err(err).
						Str("key", wrItem.key).
						Msg("Redis cache - failed to write item")
				}
			}
		}
	}()
}

func (rrc *Redis) Set(req *http.Request, resp proxy.CacheEntry, opts ...func(*proxy.CacheEntryOptions)) error {
	optsFin := new(proxy.CacheEntryOptions)
	for _, fn := range opts {
		fn(optsFin)
	}
	if proxy.ShouldWriteToCache(req, resp, optsFin) {
		cacheID := rrc.createCacheID(req, optsFin)
		var buffer bytes.Buffer
		encoder := gob.NewEncoder(&buffer)
		err := encoder.Encode(&resp)
		if err != nil {
			return err
		}
		select {
		case rrc.writeQueue <- writeQueueItem{
			key:   cacheID,
			value: buffer.Bytes(),
		}: // write OK
		default:
			return fmt.Errorf("Redis cache write queue full")
		}
	}
	return nil
}

func NewRedisCache(ctx context.Context, conf *proxy.CacheConf) *Redis {
	addr := conf.RedisAddr
	addrElms := strings.Split(":", addr)
	if len(addrElms) == 1 {
		addr = fmt.Sprintf("%s:%d", addr, defaultRedisPort)
		log.Warn().Msgf("Caching: Redis port not specified, using %d", defaultRedisPort)
	}

	ans := &Redis{
		ctx:  ctx,
		conf: conf,
		redisClient: redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   conf.RedisDB,
		}),
		writeQueue: make(chan writeQueueItem, writeChannelCapacity),
	}
	ans.goRunWriter()
	return ans
}
