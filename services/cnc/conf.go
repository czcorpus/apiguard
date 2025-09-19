// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
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

package cnc

import (
	"fmt"

	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/monitoring"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/session"

	"github.com/rs/zerolog/log"
)

type ProxyConf struct {
	// BackendURL is a URL where the backend is installed
	// (typically something like "http://192.168.1.x:8080")
	// The URL should not end with the slash character
	BackendURL string `json:"backendUrl"`

	// FrontendURL should specify a URL clients access the
	// API from. E.g. for KonText it can be something
	// like https://www.korpus.cz/kontext-api/v0.17
	// The URL should not end with the slash character
	FrontendURL string `json:"frontendUrl"`

	// FrontendSessionCookieName provides a name of the session cookie
	// used between APIGuard clients (e.g. WaG) and APIGuard.
	// If defined, APIGuard will remap this cookie to the one used
	// in the CNCAuth section where a central auth cookie is defined.
	FrontendSessionCookieName string `json:"frontendSessionCookieName"`

	// GuardType specifies guard used along with the proxy.
	// Note that some services may not provide configurable
	// guards.
	GuardType guard.GuardType `json:"guardType"`

	UseHeaderXApiKey bool `json:"useHeaderXApiKey"`

	//
	// Deprecated: The value has been replaced by TrueUserIDHeader
	UserIDPassHeader string `json:"userIdPassHeader"`

	// TrueUserIDHeader specifies a header name APIGuard will
	// use to pass detected user ID to a configured guarded
	// service. If empty then no user ID info will be passed.
	TrueUserIDHeader string `json:"trueUserIdHeader"`

	Limits []proxy.Limit `json:"limits"`

	Alarm monitoring.AlarmConf `json:"alarm"`

	ReqTimeoutSecs int `json:"reqTimeoutSecs"`

	IdleConnTimeoutSecs int `json:"idleConnTimeoutSecs"`

	SessionValType session.SessionType `json:"sessionValType"`

	// CachingPerSession allows for more granular caching (per session)
	// In case of WaG and similar situations, this should be false
	CachingPerSession bool `json:"cachingPerSession"`
}

func (c *ProxyConf) Validate(context string) error {
	if c.BackendURL == "" {
		return fmt.Errorf("%s.backendUrl is missing/empty", context)
	}
	if c.TrueUserIDHeader == "" && c.UserIDPassHeader != "" {
		log.Warn().Msg("found deprecated `userIdPassHeader`, please replace by `trueUserIdHeader`")
		c.TrueUserIDHeader = c.UserIDPassHeader
	}
	if err := c.SessionValType.Validate(); err != nil {
		return fmt.Errorf("%s.sessionValType is invalid: %w", context, err)
	}
	for i, limit := range c.Limits {
		if limit.BurstLimit == 0 {
			log.Warn().
				Int("default", limit.ReqPerTimeThreshold).
				Msgf("%s.limits[%d].burstLimit not set, using reqPerTimeThreshold", context, i)
			c.Limits[i] = proxy.Limit{
				ReqPerTimeThreshold:     limit.ReqPerTimeThreshold,
				ReqCheckingIntervalSecs: limit.ReqCheckingIntervalSecs,
				BurstLimit:              limit.ReqPerTimeThreshold,
			}
		}
	}
	return nil
}

func (c *ProxyConf) GetCoreConf() proxy.GeneralProxyConf {
	return proxy.GeneralProxyConf{
		BackendURL:          c.BackendURL,
		FrontendURL:         c.FrontendURL,
		ReqTimeoutSecs:      c.ReqTimeoutSecs,
		IdleConnTimeoutSecs: c.IdleConnTimeoutSecs,
		Limits:              c.Limits,
	}
}

type EnvironConf struct {
	CNCAuthCookie   string
	AuthTokenEntry  string
	ReadTimeoutSecs int
	ServicePath     string

	// ServiceKey is a unique id of service considering
	// possible multiple instances of the same service type
	// It looks like [conf order int]/[type] - e.g. 4/kontext
	ServiceKey string

	CNCPortalLoginURL string
	IsStreamingMode   bool
}
