// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package logging

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	WaGSessionName        = "wag.session"
	maxSessionValueLength = 64
)

func ExtractClientIP(req *http.Request) string {
	ip := req.Header.Get("x-forwarded-for")
	if ip != "" {
		return ip
	}
	return strings.Split(req.RemoteAddr, ":")[0]
}

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
	ip := ExtractClientIP(req)
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

func ExtractRequestIdentifiers(req *http.Request) (string, string) {
	ip := ExtractClientIP(req)
	session, err := req.Cookie(WaGSessionName)
	var sessionID string
	if err == nil {
		sessionID = NormalizeSessionID(session.Value)

	} else if err == http.ErrNoCookie {
		sessionID = ""

	} else {
		sessionID = ""
		log.Print("WARNING: failed to fetch session cookie - ", err)
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
