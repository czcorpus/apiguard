// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"context"

	"github.com/rs/zerolog/log"
)

type NullCache struct {
}

func (backend *NullCache) Get(req *StreamRequestJSON) (string, error) {
	return "", ErrCacheMiss
}

func NewNullCache(ctx context.Context, writes <-chan CacheWriteChunkReq) *NullCache {
	go func() {
		for {
			select {
			case <-writes:
				// NOP
			case <-ctx.Done():
				log.Info().Msg("closing streaming NullCache")
				return
			}
		}
	}()
	return &NullCache{}
}
