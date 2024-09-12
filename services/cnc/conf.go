// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package cnc

import (
	"apiguard/monitoring"
	"apiguard/proxy"
	"fmt"
)

const (
	GuardTypeDefault    GuardType = "default"
	GuardTypeSessionMap GuardType = "session-mapping"
	GuardTypeNull       GuardType = "null"
)

type GuardType string

func (gn GuardType) Validate() error {
	if gn == GuardTypeDefault || gn == GuardTypeSessionMap ||
		gn == GuardTypeNull {
		return nil
	}
	return fmt.Errorf("invalid guard name: %s", gn)
}

func (gn GuardType) String() string {
	return string(gn)
}

type ProxyConf struct {
	// InternalURL is a URL where the backend is installed
	// (typically something like "http://192.168.1.x:8080")
	// The URL should not end with the slash character
	InternalURL string `json:"internalUrl"`

	// ExternalURL should specify a URL clients access the
	// API from. E.g. for KonText it can be something
	// like https://www.korpus.cz/kontext-api/v0.17
	// The URL should not end with the slash character
	ExternalURL string `json:"externalUrl"`

	// ExternalSessionCookieName provides a name of the session cookie
	// used between APIGuard clients (e.g. WaG) and APIGuard.
	// If defined, APIGuard will remap this cookie to the one used
	// in the CNCAuth section where a central auth cookie is defined.
	ExternalSessionCookieName string `json:"externalSessionCookieName"`

	// GuardType specifies guard used along with the proxy
	GuardType GuardType `json:"guardType"`

	UseHeaderXApiKey bool `json:"useHeaderXApiKey"`

	// UserIDPassHeader specifies a header name APIGuard will
	// use to pass detected user ID to a configured guarded
	// service. If empty then no user ID info will be passed.
	UserIDPassHeader string `json:"userIdPassHeader"`

	Limits []monitoring.Limit `json:"limits"`

	Alarm monitoring.AlarmConf `json:"alarm"`

	ReqTimeoutSecs int `json:"reqTimeoutSecs"`

	IdleConnTimeoutSecs int `json:"idleConnTimeoutSecs"`
}

func (c *ProxyConf) Validate(context string) error {
	if c.InternalURL == "" {
		return fmt.Errorf("%s.internalURL is missing/empty", context)
	}
	return nil
}

func (c *ProxyConf) GetCoreConf() proxy.GeneralProxyConf {
	return proxy.GeneralProxyConf{
		InternalURL:         c.InternalURL,
		ExternalURL:         c.ExternalURL,
		ReqTimeoutSecs:      c.ReqTimeoutSecs,
		IdleConnTimeoutSecs: c.IdleConnTimeoutSecs,
	}
}

type EnvironConf struct {
	CNCAuthCookie     string
	AuthTokenEntry    string
	ReadTimeoutSecs   int
	ServiceName       string
	ServicePath       string
	CNCPortalLoginURL string
}
