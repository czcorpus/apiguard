// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

// please note that this module is mostly taken from CNC's other project "klogproc"

package monitoring

import (
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
)

// ConnectionConf specifies a configuration required to store data
// to an InfluxDB database
type ConnectionConf struct {
	Server       string `json:"server"`
	Token        string `json:"token"`
	Organization string `json:"organization"`
	Bucket       string `json:"bucket"`
}

// IsConfigured tests whether the configuration is considered
// to be enabled (i.e. no error checking just enabled/disabled)
func (conf *ConnectionConf) IsConfigured() bool {
	return conf.Server != ""
}

// ------

type Influxable interface {
	ToInfluxDB() (map[string]string, map[string]any)
	GetTime() time.Time
}

// RecordWriter is a simple wrapper around InfluxDB client allowing
// adding records in a convenient way without need to think
// about batch processing of the records. The price paid here
// is that the client is statefull and Finish() method must
// be always called to finish the current operation.
type RecordWriter[T Influxable] struct {
	influxClient influxdb2.Client
	influxAPI    influxdb2api.WriteAPI
	address      string
}

// AddRecord adds a record and if internal batch is full then
// it also stores the record to a configured database and
// measurement. Please note that without calling Finish() at
// the end of an operation, stale records may remain.
func (c *RecordWriter[T]) AddRecord(rec T) {
	tags, values := rec.ToInfluxDB()
	p := influxdb2.NewPointWithMeasurement("state")
	p.SetTime(rec.GetTime())
	for tn, tv := range tags {
		p.AddTag(tn, tv)
	}
	for field, value := range values {
		p.AddField(field, value)
	}
	c.influxAPI.WritePoint(p)
}

// NewRecordWriter is a factory function for RecordWriter
func NewRecordWriter[T Influxable](
	conf *ConnectionConf,
	onError func(err error),
) (*RecordWriter[T], error) {
	var influxClient influxdb2.Client
	var influxWriteAPI influxdb2api.WriteAPI
	var influxErrChan <-chan error
	ourErrChan := make(chan error)
	if conf.IsConfigured() {
		influxClient = influxdb2.NewClient(conf.Server, conf.Token)
		influxWriteAPI = influxClient.WriteAPI(
			conf.Organization,
			conf.Bucket,
		)
		influxErrChan = influxWriteAPI.Errors()
		go func() {
			for err := range influxErrChan {
				ourErrChan <- err
			}
		}()
	}
	go func() {
		for err := range ourErrChan {
			log.Print("ERROR: error writing data to InfluxDB: ", err)
			onError(err)
		}
	}()
	return &RecordWriter[T]{
		influxClient: influxClient,
		influxAPI:    influxWriteAPI,
		address:      conf.Server,
	}, nil
}
