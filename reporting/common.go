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
