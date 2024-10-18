// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package dflt

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/session"
	"apiguard/telemetry"
	"database/sql"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"golang.org/x/time/rate"
)

const (
	dfltCleanupInterval          = time.Duration(1) * time.Hour
	dfltNumOKRequestsPerInterval = 10
)

// Guard provides basic request protection
// based on IP counting and with some advantages
// for authenticated users.
type Guard struct {
	db                *sql.DB
	storage           telemetry.Storage
	sessionCookieName string
	numRequests       *collections.ConcurrentMap[string, guard.RequestIPCount]
	clientCounter     chan common.ClientID
	cleanupInterval   time.Duration
	loc               *time.Location
	anonymousUsers    common.AnonymousUsers
	rateLimiters      map[string]*rate.Limiter
	confLimits        []proxy.Limit
	rateLimitersMu    sync.Mutex
	sessionValFactory func() session.HTTPSession
}

func (sra *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {
	return common.InvalidUserID, nil
}

func (sra *Guard) ExposeAsCounter() chan<- common.ClientID {
	return sra.clientCounter
}

func (sra *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (guard.DelayInfo, error) {
	num := sra.numRequests.Get(clientID.GetKey())
	if num.Num <= dfltNumOKRequestsPerInterval {
		return guard.DelayInfo{}, nil
	}
	d := 10 * (1/(1+math.Pow(math.E, -float64((num.Num-dfltNumOKRequestsPerInterval))/100)) - 0.5)
	return guard.DelayInfo{
			Delay: time.Duration(d*1000) * time.Millisecond,
			IsBan: false,
		},
		nil
}

func (sra *Guard) LogAppliedDelay(respDelay guard.DelayInfo, clientID common.ClientID) error {
	if err := sra.storage.LogAppliedDelay(respDelay, clientID); err != nil {
		return err
	}
	return nil
}

func (sra *Guard) ClientInducedRespStatus(req *http.Request) guard.ReqProperties {
	if len(sra.confLimits) > 0 {
		clientIP, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
		if err != nil {
			return guard.ReqProperties{
				ProposedStatus: http.StatusUnauthorized,
				UserID:         common.InvalidUserID,
				SessionID:      "",
				Error:          fmt.Errorf("failed to determine user IP: %w", err),
			}
		}

		sra.rateLimitersMu.Lock()
		defer sra.rateLimitersMu.Unlock()
		limiter, exists := sra.rateLimiters[clientIP]
		if !exists {
			flimit := sra.confLimits[0]
			limiter = rate.NewLimiter(
				flimit.NormLimitPerSec(),
				flimit.BurstLimit,
			)
			sra.rateLimiters[clientIP] = limiter
		}
		if !limiter.Allow() {
			return guard.ReqProperties{
				ProposedStatus: http.StatusTooManyRequests,
			}
		}
	}
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
	}
}

func (sra *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return sra.anonymousUsers.IsAnonymous(userID)
}

func (sra *Guard) Run() {
	for v := range sra.clientCounter {
		key := v.GetKey()
		newVal := sra.numRequests.Get(key).Inc() // we must Inc() so time is not zero
		if newVal.CountStart.Before(time.Now().Add(-sra.cleanupInterval)) {
			newVal = guard.RequestIPCount{
				CountStart: time.Now(),
				Num:        1,
			}
		}
		sra.numRequests.Set(key, newVal)
	}
}

func New(
	globalCtx *globctx.Context,
	sessionCookieName string,
	sessionType session.SessionType,
	confLimits []proxy.Limit,
) *Guard {
	return &Guard{
		db:                globalCtx.CNCDB,
		sessionCookieName: sessionCookieName,
		numRequests:       collections.NewConcurrentMap[string, guard.RequestIPCount](),
		clientCounter:     make(chan common.ClientID),
		cleanupInterval:   dfltCleanupInterval,
		loc:               globalCtx.TimezoneLocation,
		anonymousUsers:    globalCtx.AnonymousUserIDs,
		confLimits:        confLimits,
		rateLimiters:      make(map[string]*rate.Limiter),
		sessionValFactory: guard.CreateSessionValFactory(sessionType),
	}
}
