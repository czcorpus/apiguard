// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ctx

import (
	"database/sql"
	"time"

	"github.com/czcorpus/cnc-gokit/influx"
)

// GlobalContext provides access to shared resources and information needed by different
// part of the application. It is OK to pass it by value as the properties of the struct
// are pointers themselves (if needed).
type GlobalContext struct {
	TimezoneLocation *time.Location
	BackendLogger    *BackendLogger
	CNCDB            *sql.DB
	InfluxDB         *influx.InfluxDBAdapter
}
