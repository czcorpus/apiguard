// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package influx

import (
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
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

type InfluxDBAdapter struct {
	api     influxdb2api.WriteAPI
	errChan <-chan error
	address string
}

func (db *InfluxDBAdapter) WritePoint(p *write.Point) {
	db.api.WritePoint(p)
}

func (db *InfluxDBAdapter) Address() string {
	return db.address
}

func (db *InfluxDBAdapter) OnError(handler func(error)) {
	go func() {
		for err := range db.errChan {
			handler(err)
		}
	}()
}

func ConnectAPI(conf *ConnectionConf) *InfluxDBAdapter {
	ans := new(InfluxDBAdapter)
	var influxClient influxdb2.Client
	if conf.IsConfigured() {
		ans.address = conf.Server
		influxClient = influxdb2.NewClient(conf.Server, conf.Token)
		ans.api = influxClient.WriteAPI(
			conf.Organization,
			conf.Bucket,
		)
		ans.errChan = ans.api.Errors()
		return ans
	}
	return nil
}
