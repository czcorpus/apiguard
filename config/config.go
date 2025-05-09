// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package config

import (
	"apiguard/botwatch"
	"apiguard/cnc"
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/session"
	"apiguard/telemetry"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"
)

const (
	DfltServerReadTimeoutSecs                = 10
	DfltServerWriteTimeoutSecs               = 30
	DftlServerPort                           = 8080
	DfltServerHost                           = "localhost"
	DfltBanSecs                              = 3600
	DfltTimeZone                             = "Europe/Prague"
	DfltProxyReqTimeoutSecs                  = 60
	DfltIdleConnTimeoutSecs                  = 10
	DfltSessionValType                       = session.SessionTypeCNC
	OperationModeProxy         OperationMode = "proxy"
	OperationModeStreaming     OperationMode = "streaming"
)

type GeneralServiceConf struct {
	Type string          `json:"type"`
	Conf json.RawMessage `json:"conf"`
}

type CNCAuthConf struct {
	SessionCookieName string `json:"sessionCookieName"`
}

type OperationMode string

func (om OperationMode) Validate() error {
	if om != OperationModeProxy && om != OperationModeStreaming {
		return fmt.Errorf("unknown operation mode %s (supported: proxy, streaming)", om)
	}
	return nil
}

/*
func (services *servicesSection) validate() error {
	if services.Kontext != nil {

	}
	if services.Treq != nil {
		if services.Treq.FrontendURL == "" {
			return errors.New("missing frontendUrl configuration for Treq")
		}
		if services.Treq.BackendURL == "" {
			return errors.New("missing backendUrl configuration for Treq")
		}
		if services.Treq.SessionValType == "" {
			log.Warn().Msgf(
				"missing services.treq.sessionValType, setting %s", DfltSessionValType)
			services.Treq.SessionValType = DfltSessionValType
		}
		if err := services.Treq.SessionValType.Validate(); err != nil {
			return fmt.Errorf("invalid value of treq.sessionValType: %w", err)
		}
		if services.Treq.ReqTimeoutSecs == 0 {
			services.Treq.ReqTimeoutSecs = DfltProxyReqTimeoutSecs
			log.Warn().Msgf("missing services.treq.reqTimeoutSecs, setting %d", DfltProxyReqTimeoutSecs)
		}
		if services.Treq.IdleConnTimeoutSecs == 0 {
			services.Treq.IdleConnTimeoutSecs = DfltIdleConnTimeoutSecs
			log.Warn().Msgf("missing services.treq.idleConnTimeoutSecs, setting %d", DfltIdleConnTimeoutSecs)
		}
	}
	if services.KWords != nil {
		if services.KWords.FrontendURL == "" {
			return errors.New("missing frontendUrl configuration for KWords")
		}
		if services.KWords.BackendURL == "" {
			return errors.New("missing backendUrl configuration for KWords")
		}
		if services.KWords.ReqTimeoutSecs == 0 {
			services.KWords.ReqTimeoutSecs = DfltProxyReqTimeoutSecs
			log.Warn().Msgf("missing services.kwords.reqTimeoutSecs, setting %d", DfltProxyReqTimeoutSecs)
		}
		if services.KWords.IdleConnTimeoutSecs == 0 {
			services.KWords.IdleConnTimeoutSecs = DfltIdleConnTimeoutSecs
			log.Warn().Msgf("missing services.kwords.idleConnTimeoutSecs, setting %d", DfltIdleConnTimeoutSecs)
		}
	}
	return nil
}
*/

type Configuration struct {
	apiAllowedClientsCache    []net.IPNet
	ServerHost                string        `json:"serverHost"`
	ServerPort                int           `json:"serverPort"`
	ServerReadTimeoutSecs     int           `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs    int           `json:"serverWriteTimeoutSecs"`
	TimeZone                  string        `json:"timeZone"`
	PublicRoutesURL           string        `json:"publicRoutesUrl"`
	OperationMode             OperationMode `json:"operationMode"`
	DisableStreamingModeCache bool          `json:"disableStreamingModeCache"`
	WagTilesConfDir           string        `json:"wagTilesConfDir"`

	// APIAllowedClients is a list of IP/CIDR addresses allowed to access the API.
	// Mostly, we should stick here with our internal network.
	APIAllowedClients []string                 `json:"apiAllowedClients"`
	Botwatch          botwatch.Conf            `json:"botwatch"`
	Telemetry         *telemetry.Conf          `json:"telemetry"`
	Services          []GeneralServiceConf     `json:"services"`
	Cache             *proxy.CacheConf         `json:"cache"`
	Reporting         *reporting.Conf          `json:"reporting"`
	Logging           logging.LoggingConf      `json:"logging"`
	Monitoring        *monitoring.LimitingConf `json:"monitoring"`
	IPBanTTLSecs      int                      `json:"IpBanTtlSecs"`
	CNCDB             cnc.Conf                 `json:"cncDb"`
	Mail              *monitoring.MailConf     `json:"mail"`
	CNCAuth           CNCAuthConf              `json:"cncAuth"`
	IgnoreStoredState bool                     `json:"-"`
}

func (c *Configuration) loadAPIAllowlist() error {
	c.apiAllowedClientsCache = make([]net.IPNet, 0, len(c.APIAllowedClients))
	for _, a := range c.APIAllowedClients {
		_, net, err := net.ParseCIDR(a)
		if err != nil {
			return fmt.Errorf("failed to parse allowlist element %s: %w", a, err)
		}
		c.apiAllowedClientsCache = append(c.apiAllowedClientsCache, *net)
	}
	return nil
}

func (c *Configuration) IPAllowedForAPI(ip net.IP) bool {
	if c.apiAllowedClientsCache == nil {
		if err := c.loadAPIAllowlist(); err != nil {
			panic(err)
		}
	}
	if ip == nil {
		return false
	}
	if len(c.apiAllowedClientsCache) == 0 {
		return true
	}
	for _, netw := range c.apiAllowedClientsCache {
		if netw.Contains(ip) {
			return true
		}
	}
	return false
}

func (c *Configuration) Validate() error {
	if err := c.loadAPIAllowlist(); err != nil {
		return err
	}
	if err := c.Botwatch.Validate("botwatch"); err != nil {
		return err
	}
	if c.Telemetry != nil {
		if err := c.Telemetry.Validate("telemetry"); err != nil {
			return err
		}
	}
	if err := c.CNCDB.Validate("cncDb"); err != nil {
		return err
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return err
	}
	if err := c.Monitoring.ValidateAndDefaults(); err != nil {
		return err
	}
	if err := c.Reporting.ValidateAndDefaults(); err != nil {
		return err
	}
	if err := c.OperationMode.Validate(); err != nil {
		return err
	}
	if c.WagTilesConfDir != "" {
		isDir, err := fs.IsDir(c.WagTilesConfDir)
		if err != nil {
			return fmt.Errorf("failed to test wagTilesConfDir %s: %w", c.WagTilesConfDir, err)
		}
		if !isDir {
			return fmt.Errorf("wagTilesConfDir %s is not a directory", c.WagTilesConfDir)
		}
	}

	return nil
}

func (c *Configuration) TimezoneLocation() *time.Location {
	// we can ignore the error here as we always call c.Validate()
	// first (which also tries to load the location and report possible
	// error)
	loc, _ := time.LoadLocation(c.TimeZone)
	return loc
}

func LoadConfig(path string) *Configuration {
	if path == "" {
		log.Fatal().Msg("Cannot load config - path not specified")
	}
	rawData, err := os.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	var conf Configuration
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	return &conf
}
