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

package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/czcorpus/apiguard/proxy"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// operation represents the type of Redis operation to be performed asynchronously
type operation int

const (
	operationSet    operation = 1
	operationExpire operation = 2

	defaultRedisPort     = 6379
	writeChannelCapacity = 100
)

// writeQueueItem represents a queued Redis write operation
// that will be processed asynchronously by the background writer
type writeQueueItem struct {
	operation operation // the type of operation to perform (set or expire)
	key       string    // Redis key for the operation
	expire    int       // expiration time in seconds (for expire operations)
	value     []byte    // serialized value to store (for set operations)
}

// Redis implements the proxy.Cache interface using Redis as the backing store.
// It provides asynchronous write operations through a background goroutine
// to avoid blocking HTTP request processing.
type Redis struct {
	ctx         context.Context     // context for controlling goroutine lifecycle
	conf        *proxy.CacheConf    // cache configuration settings
	redisClient *redis.Client       // Redis client connection
	writeQueue  chan writeQueueItem // channel for queuing async write operations
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
	select {
	case rrc.writeQueue <- writeQueueItem{
		operation: operationExpire,
		key:       cacheID,
		expire:    rrc.conf.TTLSecs,
	}: // write OK
	default:
		log.Error().
			Str("key", cacheID).
			Err(fmt.Errorf("Redis cache write queue full")).
			Msg("failed to set TTL for cache entry")
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
			case operation := <-rrc.writeQueue:
				switch operation.operation {
				case operationExpire:
					_, err := rrc.redisClient.Expire(rrc.ctx, operation.key, time.Duration(operation.expire)*time.Second).Result()
					if err != nil {
						log.Error().
							Err(fmt.Errorf("proxy cache access error: %w", err)).
							Str("key", operation.key).
							Msg("Redis cache - failed to execute EXPIRE")
					}
				case operationSet:
					_, err := rrc.redisClient.Set(rrc.ctx, operation.key, operation.value, time.Duration(rrc.conf.TTLSecs)*time.Second).Result()
					if err != nil {
						log.Error().
							Err(err).
							Str("key", operation.key).
							Msg("Redis cache - failed to execute SET")
					}
				default:
					log.Warn().Any("op", operation).Msg("unknown operation in Redis cache queue")
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
			operation: operationSet,
			key:       cacheID,
			value:     buffer.Bytes(),
		}: // write OK
		default:
			return fmt.Errorf("Redis cache write queue full")
		}
	}
	return nil
}

// NewRedisCache creates a new Redis cache instance and starts a background
// goroutine for processing asynchronous write operations.
func NewRedisCache(ctx context.Context, conf *proxy.CacheConf) *Redis {
	addr := conf.RedisAddr
	addrElms := strings.Split(addr, ":")
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
