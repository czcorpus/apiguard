// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

// please note that this module is mostly taken from CNC's other project "klogproc"

package monitoring

import (
	"apiguard/monitoring/influx"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// RecordWriter is a simple wrapper around InfluxDB client allowing
// adding records in a convenient way without need to think
// about batch processing of the records. The price paid here
// is that the client is statefull and Finish() method must
// be always called to finish the current operation.
type RecordWriter[T influx.Influxable] struct {
	db *influx.InfluxDBAdapter
}

// AddRecord adds a record and if internal batch is full then
// it also stores the record to a configured database and
// measurement. Please note that without calling Finish() at
// the end of an operation, stale records may remain.
func (c *RecordWriter[T]) AddRecord(rec T, measurement string) {
	tags, values := rec.ToInfluxDB()
	p := influxdb2.NewPointWithMeasurement(measurement)
	p.SetTime(rec.GetTime())
	for tn, tv := range tags {
		p.AddTag(tn, tv)
	}
	for field, value := range values {
		p.AddField(field, value)
	}
	c.db.WritePoint(p)
}

// NewRecordWriter is a factory function for RecordWriter
func NewRecordWriter[T influx.Influxable](db *influx.InfluxDBAdapter) *RecordWriter[T] {
	return &RecordWriter[T]{db}
}
