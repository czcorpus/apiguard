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
	api             influxdb2api.WriteAPI
	address         string
	onErrorHandlers []func(error)
}

func (db *InfluxDBAdapter) WritePoint(p *write.Point) {
	db.api.WritePoint(p)
}

func (db *InfluxDBAdapter) Address() string {
	return db.address
}

func ConnectAPI(conf *ConnectionConf, errListen <-chan error) *InfluxDBAdapter {
	ans := new(InfluxDBAdapter)
	ans.onErrorHandlers = make([]func(error), 0, 10)
	var influxClient influxdb2.Client
	if conf.IsConfigured() {
		ans.address = conf.Server
		influxClient = influxdb2.NewClient(conf.Server, conf.Token)
		ans.api = influxClient.WriteAPI(
			conf.Organization,
			conf.Bucket,
		)
		go func() {
			rt := make(chan error)
			for err := range ans.api.Errors() {
				rt <- err
			}
			close(rt)
		}()
		return ans
	}
	return nil
}
