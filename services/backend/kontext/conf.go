// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
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

	// SessionCookieName is defined by CNC's portal so ask a portal admin
	// for more info. Typically, this is something like 'cnc_toolbar_sid',
	// 'cnc_toolbar_sid_test'.
	SessionCookieName string `json:"sessionCookieName"`

	UseHeaderXApiKey bool `json:"useHeaderXApiKey"`

	Alarm alarms.Conf `json:"alarm"`
}

func (c *Conf) Validate(context string) error {
	if c.InternalURL == "" {
		return fmt.Errorf("%s.internalURL is missing/empty", context)
	}
	if c.SessionCookieName == "" {
		return fmt.Errorf("%s.sessionCookieName is missing/empty", context)
	}
	return nil
}
