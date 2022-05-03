// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package logging

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func extractClientIP(req *http.Request) string {
	ip := req.Header.Get("x-forwarded-for")
	fmt.Println("IP = ", ip)
	if ip != "" {
		return ip
	}
	return strings.Split(req.RemoteAddr, ":")[0]
}

type LGRequestRecord struct {
	IPAddress string
	ClientID  string
	Created   time.Time
}

func (rr *LGRequestRecord) GetClientIP() net.IP {
	return net.ParseIP(rr.IPAddress)
}

func (rr *LGRequestRecord) GetClientID() string {
	return fmt.Sprintf("%s#%s", rr.IPAddress, rr.ClientID)
}

func (rr *LGRequestRecord) GetTime() time.Time {
	return rr.Created
}

func NewLGRequestRecord(req *http.Request) *LGRequestRecord {
	ip := extractClientIP(req)
	return &LGRequestRecord{
		IPAddress: ip,
		ClientID:  fmt.Sprintf("%s#%s", ip, req.Header.Get("x-client-flag")),
		Created:   time.Now(),
	}
}

type AnyRequestRecord interface {
	GetClientIP() net.IP

	// GetClientID should return something more specific than IP (e.g. ip+fingerprint)
	GetClientID() string
	GetTime() time.Time
}
