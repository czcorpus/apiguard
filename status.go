// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/botwatch"
	"apiguard/cnc/guard"
	"apiguard/config"
	"apiguard/ctx"
	"apiguard/services/logging"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func runStatus(globalCtx ctx.GlobalContext, conf *config.Configuration, ident string) {
	delayLog := guard.NewDelayStats(globalCtx.CNCDB, conf.TimezoneLocation())
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

	telemetryAnalyzer, err := botwatch.NewAnalyzer(
		&conf.Botwatch,
		&conf.Telemetry,
		globalCtx.InfluxDB,
		delayLog,
		delayLog,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	fakeReq, err := http.NewRequest("POST", "", nil)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if sessionID != "" {
		fakeReq.AddCookie(&http.Cookie{
			Name:  logging.WaGSessionName,
			Value: sessionID,
		})
	}
	fakeReq.RemoteAddr = ip.String()

	if sessionID != "" {
		delay, err := telemetryAnalyzer.CalcDelay(fakeReq)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		botScore, err := telemetryAnalyzer.BotScore(fakeReq)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		fmt.Printf(
			"\nSession: %s"+
				"\nbot score: %01.2f"+
				"\nreq. delay: %v"+
				"\n",
			sessionID, botScore, delay,
		)

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
