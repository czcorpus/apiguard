// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
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
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

type TokenConf struct {
	HashedValue string        `json:"value"`
	UserID      common.UserID `json:"userId"`
}

// Guard in the `token` package - besides standard functions like throttling
// too high request rate and applying IP bans - allows access only to users
// with valid access tokens provided via an HTTP header.
type Guard struct {
	servicePath string

	tlmtrStorage telemetry.Storage

	anonymousUsers common.AnonymousUsers

	tokenHeaderName string

	rateLimiters map[string]*rate.Limiter

	confLimits []proxy.Limit

	rateLimitersMu sync.Mutex

	hashedTokens []TokenConf

	authExcludedPathPrefixes []string
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
			Str("guardType", "cncauth").
			Str("clientId", clientID.GetKey()).
			Msg("applied IP ban")
		return true, nil
	}
	return false, nil
}

func (g *Guard) pathMatchesExclude(req *http.Request) bool {
	for _, excl := range g.authExcludedPathPrefixes {
		tst, err := url.JoinPath(g.servicePath, excl)
		if err != nil {
			log.Error().Err(err).Msg("pathMatchesExclude failed to join service path and exclusion path")
			return false
		}
		if req.URL.Path == tst {
			return true
		}
	}
	return false
}

func (g *Guard) EvaluateRequest(req *http.Request, fallbackCookie *http.Cookie) guard.ReqEvaluation {
	userID := g.validateToken(req.Header.Get(g.tokenHeaderName))
	if !(userID.IsValid() || g.pathMatchesExclude(req)) {
		return guard.ReqEvaluation{
			ProposedResponse: http.StatusUnauthorized,
			ClientID:         common.InvalidUserID,
			SessionID:        "",
			Error:            fmt.Errorf("invalid authentication token"),
		}
	}
	clientIP := proxy.ExtractClientIP(req)

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
			log.Debug().Str("clientIp", clientIP).Msg("limiting client with status 429")
			return guard.ReqEvaluation{
				ProposedResponse: http.StatusTooManyRequests,
				ClientID:         userID,
				SessionID:        "",
			}
		}
	}

	// test ip ban
	banned, err := g.checkForBan(req, common.ClientID{IP: clientIP, ID: userID})
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
		ClientID:         userID,
		SessionID:        "",
		Error:            err,
	}
}

func (g *Guard) TestUserIsAnonymous(userID common.UserID) bool {
	return g.anonymousUsers.IsAnonymous(userID)
}

func (g *Guard) DetermineTrueUserID(req *http.Request) (common.UserID, error) {
	userID := g.validateToken(req.Header.Get(g.tokenHeaderName))
	return userID, nil
}

// NewGuard creates a new token guard.
// The `authExcludedPathPrefixes` should not contain a leading slash
// as they are used relative to respective APIGuard's service URL path.
// E.g. to exclude endpoint `/openapi`, one defines the argument
// as []string{"openapi"} and APIGuard adds that to respective service
// URL path - e.g. `/service/3/mquery/openapi`
func NewGuard(
	globalCtx *globctx.Context,
	servicePath string,
	tokenHeaderName string,
	confLimits []proxy.Limit,
	hashedTokens []TokenConf,
	authExcludedPathPrefixes []string,
) *Guard {
	return &Guard{
		servicePath:              servicePath,
		tlmtrStorage:             globalCtx.TelemetryDB,
		anonymousUsers:           globalCtx.AnonymousUserIDs,
		tokenHeaderName:          tokenHeaderName,
		confLimits:               confLimits,
		rateLimiters:             make(map[string]*rate.Limiter),
		hashedTokens:             hashedTokens,
		authExcludedPathPrefixes: authExcludedPathPrefixes,
	}
}
