// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
	"apiguard/services/backend"
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

	// CookieMapping enables management of anonymous users
	// by employing a fallback user (i.e., a user recognized
	// by the CNC portal and capable of authenticating with APIs
	// registered on the portal) while concealing this fallback
	// user. This is essential because, if the original cookie
	// were used, the portal logic (which is a component of the
	// client page) would display the user information - which is
	// a thing we do not want.
	//
	// To put this in different words - we have an API instance
	// e.g. KonText which prefers CNC cnc_toolbar_sid cookie.
	// Our web client has integrated CNC toolbar which provides
	// a way to authenticate and display logged in user info.
	// But at the same time we want anonymous users to be
	// authenticated via the fallback users without toolbar knowing
	// this. So our web client app will create an additional cookie
	// only it and APIGuard will understand and APIGuard will replace
	// the cookie name using this mapping to make sure (KonText) API
	// understands it (as it requires cnc_toolbar_sid).
	CookieMapping backend.CookieMapping `json:"cookieMapping"`

	UseHeaderXApiKey bool `json:"useHeaderXApiKey"`

	Limits []alarms.Limit `json:"limits"`

	Alarm alarms.AlarmConf `json:"alarm"`
}

func (c *Conf) Validate(context string) error {
	if c.InternalURL == "" {
		return fmt.Errorf("%s.internalURL is missing/empty", context)
	}
	return nil
}
