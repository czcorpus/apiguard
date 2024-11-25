// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package treq

import (
	"apiguard/monitoring"
	"apiguard/proxy"
	"apiguard/session"
	"fmt"
)

type Conf struct {
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

	Limits []proxy.Limit `json:"limits"`

	Alarm monitoring.AlarmConf `json:"alarm"`

	ReqTimeoutSecs int `json:"reqTimeoutSecs"`

	IdleConnTimeoutSecs int `json:"idleConnTimeoutSecs"`

	SessionValType session.SessionType `json:"sessionValType"`
}

func (c *Conf) Validate(context string) error {
	if c.BackendURL == "" {
		return fmt.Errorf("%s.backendUrl is missing/empty", context)
	}
	if err := c.SessionValType.Validate(); err != nil {
		return fmt.Errorf("%s.sessionValType is invalid: %w", context, err)
	}
	return nil
}

func (c *Conf) GetCoreConf() proxy.GeneralProxyConf {
	return proxy.GeneralProxyConf{
		BackendURL:          c.BackendURL,
		FrontendURL:         c.FrontendURL,
		ReqTimeoutSecs:      c.ReqTimeoutSecs,
		IdleConnTimeoutSecs: c.IdleConnTimeoutSecs,
		Limits:              c.Limits,
	}
}
