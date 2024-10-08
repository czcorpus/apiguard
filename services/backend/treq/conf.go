// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/monitoring"
	"apiguard/proxy"
	"fmt"
)

type Conf struct {
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

	Limits []proxy.Limit `json:"limits"`

	Alarm monitoring.AlarmConf `json:"alarm"`

	ReqTimeoutSecs int `json:"reqTimeoutSecs"`

	IdleConnTimeoutSecs int `json:"idleConnTimeoutSecs"`
}

func (c *Conf) Validate(context string) error {
	if c.InternalURL == "" {
		return fmt.Errorf("%s.internalURL is missing/empty", context)
	}
	return nil
}

func (c *Conf) GetCoreConf() proxy.GeneralProxyConf {
	return proxy.GeneralProxyConf{
		InternalURL:         c.InternalURL,
		ExternalURL:         c.ExternalURL,
		ReqTimeoutSecs:      c.ReqTimeoutSecs,
		IdleConnTimeoutSecs: c.IdleConnTimeoutSecs,
		Limits:              c.Limits,
	}
}
