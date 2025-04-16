// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"github.com/gin-gonic/gin"
)

//

type StreamingCache interface {
	Get(req *StreamRequestJSON) (string, error)
}

// ------------------

// MainRespWriter handles writing of the merged
// response stream back to the client. It is basically
// the default gin.ResponseWriter with added writing to cache
type MainRespWriter struct {
	gin.ResponseWriter

	// CacheWriteKey specifies cache key under which writer's
	// data will be stored to cache. If empty, then the writer
	// won't attempt to store anything
	CacheWriteKey string

	// CacheTag is stored along with cached data and is intended
	// for making stats and deciding what to keep in cache
	// in the long term.
	CacheTag CacheTag

	CacheWrites chan<- CacheWriteChunkReq
}

func (writer *MainRespWriter) Write(data []byte) (int, error) {
	if writer.CacheWriteKey != "" {
		writer.CacheWrites <- CacheWriteChunkReq{
			Data: data,
			Key:  writer.CacheWriteKey,
		}
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *MainRespWriter) WriteString(data string) (int, error) {
	if writer.CacheWriteKey != "" {
		writer.CacheWrites <- CacheWriteChunkReq{
			Data: []byte(data),
			Tag:  writer.CacheTag,
			Key:  writer.CacheWriteKey,
		}
	}
	return writer.ResponseWriter.WriteString(data)
}

func (writer *MainRespWriter) FinishCaching() {
	if writer.CacheWriteKey != "" {
		writer.CacheWrites <- CacheWriteChunkReq{
			Flush: true,
			Key:   writer.CacheWriteKey,
		}
	}
}

func (writer *MainRespWriter) Flush() {
	writer.ResponseWriter.Flush()
}
