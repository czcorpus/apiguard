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

package main

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-common/logging"
	"github.com/czcorpus/apiguard-ext/storage"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/tlmtr"

	"github.com/rs/zerolog/log"
)

func runStatus(globalCtx *globctx.Context, conf *config.Configuration, ident string) {
	delayLog := storage.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
	ip := net.ParseIP(ident)
	var sessionID string
	if ip == nil {
		var err error
		log.Info().Msgf("assuming %s is a session ID", ident)
		sessionID = logging.NormalizeSessionID(ident)
		ip, err = delayLog.GetSessionIP(sessionID)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		if ip == nil {
			log.Fatal().Msgf("no IP address found for session %s", sessionID)
		}
	}

	telemetryAnalyzer, err := tlmtr.New(
		globalCtx,
		&conf.Botwatch,
		conf.Telemetry,
	)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	fakeReq, err := http.NewRequest("POST", "", nil)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	if sessionID != "" {
		fakeReq.AddCookie(&http.Cookie{
			Name:  logging.WaGSessionName,
			Value: sessionID,
		})
	}
	fakeReq.RemoteAddr = ip.String()

	if sessionID != "" {
		clientID := common.ClientID{
			IP: ip.String(),
			ID: common.InvalidUserID,
		}
		delay, err := telemetryAnalyzer.CalcDelay(fakeReq, clientID)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		botAnalyzer, ok := telemetryAnalyzer.(guard.BotAnalyzer)
		if ok {
			botScore, err := botAnalyzer.BotScore(fakeReq)
			if err != nil {
				log.Fatal().Err(err).Send()
			}
			fmt.Printf(
				"\nSession: %s"+
					"\nbot score: %01.2f"+
					"\nreq. delay: %v"+
					"\n",
				sessionID, botScore, delay,
			)

		} else {
			log.Fatal().
				Msgf(
					"telemetryAnalyzer %s does not implement BotAnalyzer interface",
					reflect.TypeOf(telemetryAnalyzer),
				)
		}

	} else {
		ipStats, err := delayLog.LoadIPStats(ip.String(), conf.Telemetry.MaxAgeSecsRelevant)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		d := time.Now().Add(-time.Duration(conf.Botwatch.WatchedTimeWindowSecs) * time.Second)
		fmt.Println(" ", d)
		fmt.Printf(
			"\nShowing stats starting from: %s"+
				"\nIP: %s"+
				"\nNumber of requests: %d"+
				"\nRequests stdev: %01.3f"+
				"\n",
			d, ip.String(), ipStats.Count, ipStats.Stdev(),
		)
	}
}
