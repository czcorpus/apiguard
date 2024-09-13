// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package reporting

import (
	"time"

	"github.com/czcorpus/hltscl"
)

// Timescalable represents any type which is able
// to export its data in a format required by TimescaleDB writer.
type Timescalable interface {

	// ToTimescaleDB defines a method providing data
	// to be written to a database. The first returned
	// value is for tags, the second one for fields.
	ToTimescaleDB(tableWriter *hltscl.TableWriter) *hltscl.Entry

	// GetTime provides a date and time when the record
	// was created.
	GetTime() time.Time

	// GetTableName provides a destination table name
	GetTableName() string

	// MarshalJSON provides a way how to convert the value into JSON.
	// In APIGuard, this is mostly used for logging and debugging.
	MarshalJSON() ([]byte, error)
}

type ReportingWriter interface {
	LogErrors()
	Write(item Timescalable)
	AddTableWriter(tableName string)
}
