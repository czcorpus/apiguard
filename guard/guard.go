// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package guard

import (
	"apiguard/common"
	"fmt"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

const (
	// UltraDuration is a reasonably high request delay which
	// can be considered an "infinite wait".
	UltraDuration = time.Duration(24) * time.Hour
)

type DelayInfo struct {
	Delay time.Duration
	IsBan bool
}

type RequestInfo struct {
	Created     time.Time     `json:"created"`
	Service     string        `json:"service"`
	NumRequests int           `json:"numRequests"`
	IP          string        `json:"ip"`
	UserID      common.UserID `json:"userId"`
}

type RequestIPCount struct {
	CountStart time.Time
	Num        int
}

func (ipc RequestIPCount) Inc() RequestIPCount {
	cs := ipc.CountStart
	if cs.IsZero() {
		cs = time.Now()
	}
	return RequestIPCount{
		CountStart: cs,
		Num:        ipc.Num + 1,
	}
}

type ReqProperties struct {
	// UserID is a user ID used to access the API
	// In general it can be true user ID or some replacement
	// for a specific application (e.g. WaG as a whole uses a single
	// ID)
	UserID         common.UserID
	SessionID      string
	ProposedStatus int
	Error          error
}

func (rp ReqProperties) ForbidsAccess() bool {
	return rp.ProposedStatus >= 400 && rp.ProposedStatus < 500
}

// ServiceGuard is an object which helps a proxy to decide
// how to deal with an incoming message in terms of
// authentication, throttling or even banning.
type ServiceGuard interface {

	// CalcDelay calculates how long should be the current
	// request delayed based on request properties.
	// Ideally, this is zero for a new or good behaving client.
	CalcDelay(req *http.Request, clientID common.ClientID) (DelayInfo, error)

	// LogAppliedDelay should store information about applied delay for future
	// delay calculations (for the same client)
	LogAppliedDelay(respDelay DelayInfo, clientID common.ClientID) error

	ClientInducedRespStatus(req *http.Request) ReqProperties

	TestUserIsAnonymous(userID common.UserID) bool

	DetermineTrueUserID(req *http.Request) (common.UserID, error)
}

func RestrictResponseTime(
	w http.ResponseWriter,
	req *http.Request,
	readTimeoutSecs int,
	guard ServiceGuard,
	client common.ClientID,
) error {
	respDelay, err := guard.CalcDelay(req, client)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			w,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return fmt.Errorf("failed to restrict response time: %w", err)
	}
	log.Debug().Msgf("Client is going to wait for %v", respDelay)
	if respDelay.Delay.Seconds() >= float64(readTimeoutSecs) {
		uniresp.WriteJSONErrorResponse(
			w,
			uniresp.NewActionError("service overloaded"),
			http.StatusServiceUnavailable,
		)
		return fmt.Errorf("failed to restrict response time: %w", err)
	}
	if err := guard.LogAppliedDelay(respDelay, client); err != nil {
		uniresp.WriteJSONErrorResponse(
			w,
			uniresp.NewActionError("service handling error: %s", err),
			http.StatusInternalServerError,
		)
		return fmt.Errorf("failed to restrict response time: %w", err)
	}
	time.Sleep(respDelay.Delay)
	return nil
}
