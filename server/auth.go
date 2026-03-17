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

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/czcorpus/apiguard/config"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func authTokenMatches(stored, provided string) bool {
	if hashed, ok := strings.CutPrefix(stored, "sha256:"); ok {
		sum := sha256.Sum256([]byte(provided))
		return hex.EncodeToString(sum[:]) == hashed
	}
	return stored == provided
}

func isLocalNetwork(conf *config.Configuration, ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if len(conf.Auth.LocalNetworks) > 0 {
		for _, cidr := range conf.Auth.LocalNetworks {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				log.Error().Err(err).Str("cidr", cidr).Msg("invalid localNetworks entry")
				continue
			}
			if network.Contains(parsed) {
				return true
			}
		}
		return false
	}
	return ip == conf.ServerHost
}

func isKnownProxy(conf *config.Configuration, ip string) bool {
	for _, p := range conf.Auth.KnownProxies {
		if p == ip {
			return true
		}
	}
	return false
}

func AuthRequired(conf *config.Configuration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		remoteIP, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
		isLocalDirect := err == nil && isLocalNetwork(conf, remoteIP) && !isKnownProxy(conf, remoteIP)
		if !isLocalDirect {
			provided := ctx.GetHeader(conf.Auth.TokenHeaderName)
			authorized := false
			for _, stored := range conf.Auth.Tokens {
				if authTokenMatches(stored, provided) {
					authorized = true
					break
				}
			}
			if !authorized {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}
		ctx.Next()
	}
}
