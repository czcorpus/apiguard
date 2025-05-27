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

type GuardType string

const (
	// UltraDuration is a reasonably high request delay which
	// can be considered an "infinite wait".
	UltraDuration = time.Duration(24) * time.Hour

	GuardTypeNull    GuardType = "null"
	GuardTypeDflt    GuardType = "dflt"
	GuardTypeCNCAuth GuardType = "cncauth"
	GuardTypeToken   GuardType = "token"
)

func (gt GuardType) IsValid() bool {
	return gt == GuardTypeNull || gt == GuardTypeDflt || gt == GuardTypeCNCAuth ||
		gt == GuardTypeToken
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

type ReqEvaluation struct {
	// ClientID is a user ID used to access the API
	// In general it can be true user ID or some replacement
	// for a specific application (e.g. WaG as a whole uses a single
	// ID)
	ClientID common.UserID
	// HumanID specifies true end user ID
	// In general, this may or may not be available
	HumanID          common.UserID
	SessionID        string
	ProposedResponse int
	Error            error

	// UsesFallbackCookie can be set by an evaluation process
	// in case it succeeded to authenticate request against
	// the backend using the fallback (aka "one cookie for all")
	// cookie.
	// In case the value is true, it should always mean that:
	// 1)  the evaluation process has already tried the user cookies
	//     and failed.
	// AND 2) the fallback cookie is defined
	// Note that the 'true' value does not imply the evalutation
	// will propose status 200.
	UsesFallbackCookie bool
}

func (rp ReqEvaluation) ForbidsAccess() bool {
	return rp.ProposedResponse >= 400 && rp.ProposedResponse < 500
}

// ServiceGuard is an object which helps a proxy to decide
// how to deal with an incoming message in terms of
// authentication, throttling or even banning.
type ServiceGuard interface {

	// CalcDelay calculates how long should be the current
	// request delayed based on request properties.
	// Ideally, this is zero for a new or good behaving client.
	CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error)

	// LogAppliedDelay should store information about applied delay for future
	// delay calculations (for the same client)
	LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error

	// EvaluateRequest is expected to analyze the request and based
	// on:
	//  * guard's local information (e.g. recent requests from the same IP etc.),
	//  * IP ban database
	//  * authentication
	//  * etc.
	// ... it should determine which response to return along with
	// some additional info (user ID, session ID - if available/applicable)
	EvaluateRequest(req *http.Request, fallbackCookie *http.Cookie) ReqEvaluation

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
	if respDelay.Seconds() >= float64(readTimeoutSecs) {
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
	time.Sleep(respDelay)
	return nil
}
