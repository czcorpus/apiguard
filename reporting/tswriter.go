// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Department of Linguistics,
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

package reporting

import (
	"context"
	"time"

	"github.com/czcorpus/hltscl"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type Table struct {
	writer    *hltscl.TableWriter
	opsDataCh chan<- hltscl.Entry
	errCh     <-chan hltscl.WriteError
}

type TimescaleDBWriter struct {
	ctx    context.Context
	tz     *time.Location
	conn   *pgxpool.Pool
	tables map[string]*Table
}

func (sw *TimescaleDBWriter) LogErrors() {
	for name, table := range sw.tables {
		go func(name string, table *Table) {
			for {
				select {
				case <-sw.ctx.Done():
					log.Info().Msgf("about to close %s status writer", name)
					return
				case err, ok := <-table.errCh:
					if ok {
						log.Error().
							Err(err.Err).
							Str("entry", err.Entry.String()).
							Msg("error writing data to TimescaleDB")
					}
				}
			}
		}(name, table)
	}
}

func (sw *TimescaleDBWriter) Write(item Timescalable) {
	table, ok := sw.tables[item.GetTableName()]
	log.Debug().
		Float64("timeout", table.writer.CurrentQueryTimeout().Seconds()).
		Str("table", item.GetTableName()).
		Msg("writing record to TimescaleDB")
	if ok {
		table.opsDataCh <- *item.ToTimescaleDB(table.writer)

	} else {
		log.Warn().Str("table_name", item.GetTableName()).Msg("Undefined table name in writer")
	}
}

func (sw *TimescaleDBWriter) AddTableWriter(tableName string) {
	twriter := hltscl.NewTableWriter(sw.conn, tableName, "time", sw.tz)
	opsDataCh, errCh := twriter.Activate(
		sw.ctx, hltscl.WithTimeout(10*time.Second))
	sw.tables[tableName] = &Table{
		writer:    twriter,
		opsDataCh: opsDataCh,
		errCh:     errCh,
	}
}

func NewReportingWriter(connection *pgxpool.Pool, tz *time.Location, ctx context.Context) *TimescaleDBWriter {
	return &TimescaleDBWriter{
		ctx:    ctx,
		tz:     tz,
		conn:   connection,
		tables: make(map[string]*Table),
	}
}
