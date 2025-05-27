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
	"apiguard/services/logging"
	"apiguard/session"
	"apiguard/telemetry"
	"database/sql"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	dfltCleanupInterval = time.Duration(1) * time.Hour
)

// Guard provides basic request protection
// based on IP counting and with some advantages
// for authenticated users.
type Guard struct {
	db                *sql.DB
	storage           telemetry.Storage
	sessionCookieName string
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

func (sra *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error) {
	return 0, nil
}

func (sra *Guard) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	if err := sra.storage.LogAppliedDelay(respDelay, clientID); err != nil {
		return err
	}
	return nil
}

func (sra *Guard) checkForBan(req *http.Request, clientID common.ClientID) (bool, error) {
	ip, _ := logging.ExtractRequestIdentifiers(req)
	isBanned, err := sra.storage.TestIPBan(net.ParseIP(ip))
	if err != nil {
		return isBanned, err
	}
	if isBanned {
		log.Debug().
			Str("guardType", "dflt").
			Str("clientId", clientID.GetKey()).
			Msg("applied IP ban")
		return true, nil
	}
	return false, nil
}

func (sra *Guard) EvaluateRequest(req *http.Request, fallbackCookie *http.Cookie) guard.ReqEvaluation {
	clientIP := proxy.ExtractClientIP(req)

	if len(sra.confLimits) > 0 {
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
			log.Debug().Str("clientIp", clientIP).Msg("limiting client with status 429")
			return guard.ReqEvaluation{
				ProposedResponse: http.StatusTooManyRequests,
			}
		}
	}
	banned, err := sra.checkForBan(req, common.ClientID{IP: clientIP, ID: common.InvalidUserID})
	if err != nil {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusInternalServerError,
			Error:            err,
		}
	}
	if banned {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusForbidden,
		}
	}
	return guard.ReqEvaluation{
		ProposedResponse: http.StatusOK,
	}
}

func (sra *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return sra.anonymousUsers.IsAnonymous(userID)
}

func (sra *Guard) Run() {
	for range sra.clientCounter {
		// NOP, but we must read the channel to prevent infinite hang
		// on the proxy side which wants to push information no matter
		// which guard type it deals with
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
		storage:           globalCtx.TelemetryDB,
		sessionCookieName: sessionCookieName,
		clientCounter:     make(chan common.ClientID),
		cleanupInterval:   dfltCleanupInterval,
		loc:               globalCtx.TimezoneLocation,
		anonymousUsers:    globalCtx.AnonymousUserIDs,
		confLimits:        confLimits,
		rateLimiters:      make(map[string]*rate.Limiter),
		sessionValFactory: guard.CreateSessionValFactory(sessionType),
	}
}
