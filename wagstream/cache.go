// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/rs/zerolog/log"
)

var ErrCacheMiss = errors.New("streaming cache miss")

/*

CREATE TABLE apiguard_persistent_cache (
	id VARCHAR(50),
	value MEDIUMTEXT,
	tag JSON,
	created DATETIME NOT NULL,
	num_used INT NOT NULL DEFAULT 1,
	last_used DATETIME NOT NULL,
	PRIMARY KEY (id)
)

*/

type CacheTagUserQuery struct {
	Word  string   `json:"word"`
	PoS   []string `json:"pos"`
	Lemma string   `json:"lemma"`
}

type CacheTag struct {
	Queries      []CacheTagUserQuery `json:"queries"`
	QueryType    string              `json:"queryType"`
	Query1Domain string              `json:"query1Domain"`
	Query2Domain string              `json:"query2Domain"`
}

// -------------------------------

type MariaDBCacheRow struct {
	ID       string
	Value    string
	Tag      string
	Created  time.Time
	LastUsed time.Time
	NumUsed  int
}

// ------------------------------

type CacheWriteChunkReq struct {
	Data     []byte
	Key      string
	Tag      CacheTag
	Flush    bool
	Received time.Time
}

// ------------------------------

type accessInfo struct {
	id string
	dt time.Time
}

// ------------------------------

type writeChunksReqs map[string]CacheWriteChunkReq

func (wch writeChunksReqs) appendToExisting(value CacheWriteChunkReq) CacheWriteChunkReq {
	curr, ok := wch[value.Key]
	if !ok {
		wch[value.Key] = value

	} else {
		curr.Data = append(curr.Data, value.Data...)
		curr.Flush = curr.Flush || value.Flush // we need to make sure Flush won't get overwritten
		curr.Received = value.Received
		if len(curr.Tag.Queries) == 0 {
			curr.Tag = value.Tag
		}
		wch[value.Key] = curr
	}
	return wch[value.Key]
}

// ------------------------------

type PersistentCache struct {
	conn          *sql.DB
	accessUpdates chan accessInfo
	writes        <-chan CacheWriteChunkReq
	buffer        writeChunksReqs
}

func (backend *PersistentCache) listenAndWriteAccesses(ctx context.Context) {
	for {
		select {
		case access := <-backend.accessUpdates:
			if _, err := backend.conn.ExecContext(
				ctx,
				"UPDATE apiguard_persistent_cache"+
					" SET num_used = num_used + 1,"+
					" last_used = ?"+
					" WHERE id = ?",
				access.dt,
				access.id,
			); err != nil {
				log.Error().
					Err(err).
					Str("id", access.id).
					Msg("failed to update cache access stats")
			}
		case entry := <-backend.writes:
			// TODO remove old recs
			mergedEntry := backend.buffer.appendToExisting(entry)
			if mergedEntry.Flush {
				if err := backend.set(mergedEntry); err != nil {
					log.Error().Err(err).Msg("failed to insert cache record")
				}
			}
		case <-ctx.Done():
			log.Warn().Msg("stopping MariaDB cache stats updater")
			return
		}
	}
}

func (backend *PersistentCache) Get(req *StreamRequestJSON) (string, error) {
	cacheID := req.ToCacheKey()
	row := backend.conn.QueryRow(
		"SELECT id, value, tag, created, num_used, last_used "+
			"FROM apiguard_persistent_cache "+
			"WHERE id = ?", cacheID)
	var rec MariaDBCacheRow
	err := row.Scan(
		&rec.ID,
		&rec.Value,
		&rec.Tag,
		&rec.Created,
		&rec.NumUsed,
		&rec.LastUsed,
	)
	if err == sql.ErrNoRows {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("proxy cache access error: %w", err)
	}
	backend.accessUpdates <- accessInfo{
		id: cacheID,
		dt: time.Now(),
	}
	return rec.Value, nil
}

func (backend *PersistentCache) set(req CacheWriteChunkReq) error {
	cacheID := req.Key
	dt := time.Now()
	tagJSON, err := sonic.Marshal(req.Tag)
	if err != nil {
		return fmt.Errorf("failed to serialize search tag to JSON: %w", err)
	}

	if _, err := backend.conn.Exec(
		"INSERT INTO apiguard_persistent_cache (id, value, tag, created, num_used, last_used)"+
			" VALUES (?, ?, ?, ?, ?, ?)"+
			" ON DUPLICATE KEY UPDATE"+
			" value = CONCAT(value, VALUES(value))",
		cacheID, string(req.Data), tagJSON, dt, 1, dt,
	); err != nil {
		return fmt.Errorf("failed to store proxy response to cache: %w", err)
	}
	return nil
}

func NewCache(
	ctx context.Context,
	writes <-chan CacheWriteChunkReq,
	conn *sql.DB,
) *PersistentCache {
	ans := &PersistentCache{
		conn:          conn,
		accessUpdates: make(chan accessInfo, 500),
		writes:        writes,
		buffer:        make(writeChunksReqs),
	}
	go ans.listenAndWriteAccesses(ctx)
	return ans
}
