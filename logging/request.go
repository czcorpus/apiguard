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
	Created   time.Time
}

func (rr *LGRequestRecord) GetClientIP() net.IP {
	return net.ParseIP(rr.IPAddress)
}

func (rr *LGRequestRecord) GetTime() time.Time {
	return rr.Created
}

func NewLGRequestRecord(req *http.Request) *LGRequestRecord {
	return &LGRequestRecord{
		IPAddress: extractClientIP(req),
		Created:   time.Now(),
	}
}

type AnyRequestRecord interface {
	GetClientIP() net.IP
	GetTime() time.Time
}
