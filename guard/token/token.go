// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package token

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/services/logging"
	"apiguard/telemetry"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

type TokenConf struct {
	HashedValue string        `json:"value"`
	UserID      common.UserID `json:"userId"`
}

type Guard struct {
	tlmtrStorage telemetry.Storage

	anonymousUsers common.AnonymousUsers

	tokenHeaderName string

	rateLimiters map[string]*rate.Limiter

	confLimits []proxy.Limit

	rateLimitersMu sync.Mutex

	hashedTokens []TokenConf
}

func (g *Guard) CalcDelay(req *http.Request, clientID common.ClientID) (time.Duration, error) {
	return 0, nil
}

func (g *Guard) LogAppliedDelay(respDelay time.Duration, clientID common.ClientID) error {
	return g.tlmtrStorage.LogAppliedDelay(respDelay, clientID)
}

func (g *Guard) validateToken(token string) common.UserID {
	if token == "" {
		return common.InvalidUserID
	}
	hToken := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	for _, tk := range g.hashedTokens {
		if tk.HashedValue == hToken {
			return tk.UserID
		}
	}
	return common.InvalidUserID
}

func (g *Guard) checkForBan(req *http.Request, clientID common.ClientID) (bool, error) {
	ip, _ := logging.ExtractRequestIdentifiers(req)
	isBanned, err := g.tlmtrStorage.TestIPBan(net.ParseIP(ip))
	if err != nil {
		return isBanned, err
	}
	if isBanned {
		log.Debug().
			Str("guardType", "sessionmap").
			Str("clientId", clientID.GetKey()).
			Msg("applied IP ban")
		return true, nil
	}
	return false, nil
}

func (g *Guard) ClientInducedRespStatus(req *http.Request) guard.ReqProperties {
	userID := g.validateToken(req.Header.Get(g.tokenHeaderName))
	if !userID.IsValid() {
		return guard.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			ClientID:       common.InvalidUserID,
			SessionID:      "",
			Error:          fmt.Errorf("invalid authentication token"),
		}
	}

	clientIP, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		return guard.ReqProperties{
			ProposedStatus: http.StatusUnauthorized,
			ClientID:       common.InvalidUserID,
			SessionID:      "",
			Error:          fmt.Errorf("failed to determine user IP: %w", err),
		}
	}

	if len(g.confLimits) > 0 {
		g.rateLimitersMu.Lock()
		defer g.rateLimitersMu.Unlock()
		limiter, exists := g.rateLimiters[clientIP]
		if !exists {
			flimit := g.confLimits[0]
			limiter = rate.NewLimiter(
				flimit.NormLimitPerSec(),
				flimit.BurstLimit,
			)
			g.rateLimiters[clientIP] = limiter
		}
		if !limiter.Allow() {
			return guard.ReqProperties{
				ProposedStatus: http.StatusTooManyRequests,
				ClientID:       userID,
				SessionID:      "",
			}
		}
	}

	// test ip ban
	banned, err := g.checkForBan(req, common.ClientID{IP: clientIP, ID: userID})
	if err != nil {
		return guard.ReqProperties{
			ProposedStatus: http.StatusInternalServerError,
			Error:          err,
		}
	}
	if banned {
		return guard.ReqProperties{
			ProposedStatus: http.StatusForbidden,
		}
	}
	return guard.ReqProperties{
		ProposedStatus: http.StatusOK,
		ClientID:       userID,
		SessionID:      "",
		Error:          err,
	}
}

func (g *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return g.anonymousUsers.IsAnonymous(userID)
}

func (g *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {
	userID := g.validateToken(req.Header.Get(g.tokenHeaderName))
	return userID, nil
}

func NewGuard(
	globalCtx *globctx.Context,
	tokenHeaderName string,
	confLimits []proxy.Limit,
	hashedTokens []TokenConf,
) *Guard {
	return &Guard{
		tlmtrStorage:    globalCtx.TelemetryDB,
		anonymousUsers:  globalCtx.AnonymousUserIDs,
		tokenHeaderName: tokenHeaderName,
		confLimits:      confLimits,
		rateLimiters:    make(map[string]*rate.Limiter),
		hashedTokens:    hashedTokens,
	}
}
