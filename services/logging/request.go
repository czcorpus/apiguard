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

package logging

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/czcorpus/apiguard/proxy"

	"github.com/rs/zerolog/log"
)

const (
	WaGSessionName        = "wag.session"
	maxSessionValueLength = 64
)

func NormalizeSessionID(sid string) string {
	if len(sid) <= maxSessionValueLength {
		return sid
	}
	return sid[:maxSessionValueLength]
}

type LGRequestRecord struct {
	IPAddress string
	SessionID string
	Created   time.Time
}

func (rr *LGRequestRecord) GetClientIP() net.IP {
	return net.ParseIP(rr.IPAddress)
}

func (rr *LGRequestRecord) GetSessionID() string {
	return rr.SessionID
}

func (rr *LGRequestRecord) GetClientID() string {
	return fmt.Sprintf("%s#%s", rr.IPAddress, rr.SessionID)
}

func (rr *LGRequestRecord) GetTime() time.Time {
	return rr.Created
}

func NewLGRequestRecord(req *http.Request) *LGRequestRecord {
	ip := proxy.ExtractClientIP(req)
	session, err := req.Cookie(WaGSessionName)
	var sessionID string
	if err == nil {
		sessionID = NormalizeSessionID(session.Value)
	}
	return &LGRequestRecord{
		IPAddress: ip,
		SessionID: sessionID,
		Created:   time.Now(),
	}
}

// ExtractRequestIdentifiers fetches IP address of a requesting client
// and also a related session ID. In case there is no session ID, the
// function does not consider it as an error and return just an empty
// string as the second value.
func ExtractRequestIdentifiers(req *http.Request) (string, string) {
	ip := proxy.ExtractClientIP(req)
	session, err := req.Cookie(WaGSessionName)
	var sessionID string
	if err == nil {
		sessionID = NormalizeSessionID(session.Value)

	} else if err == http.ErrNoCookie {
		sessionID = ""

	} else {
		sessionID = ""
		log.Warn().Err(err).Msg("failed to fetch session cookie - ")
	}
	return ip, sessionID
}

type AnyRequestRecord interface {
	GetClientIP() net.IP
	GetSessionID() string
	// GetClientID should return something more specific than IP (e.g. ip+fingerprint)
	GetClientID() string
	GetTime() time.Time
}
