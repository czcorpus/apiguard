// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

import (
	"apiguard/monitoring"
	"apiguard/services"
	"apiguard/services/logging"
	"apiguard/services/telemetry"
	"apiguard/services/telemetry/backend"
	"apiguard/services/telemetry/backend/counting"
	"apiguard/services/telemetry/backend/dumb"
	"apiguard/services/telemetry/backend/entropy"
	"apiguard/services/telemetry/backend/neural"
	"fmt"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	ultraDuration = time.Duration(24) * time.Hour
)

type Backend interface {
	Learn() error

	// BotScore should evaluate client legitimacy using
	// interval 0 to 1 where:
	// * 0 = perflectly legit client (= no bot)
	// * 1 = super-likely bot
	// In case the returned error is ErrUnknownClient
	BotScore(req *http.Request) (float64, error)
}

type StatsStorage interface {
	LoadStats(clientIP, sessionID string, maxAgeSecs int, insertIfNone bool) (*IPProcData, error)
	LoadIPStats(clientIP string, maxAgeSecs int) (*IPAggData, error)
	TestIPBan(IP net.IP) (bool, error)
	RegisterDelayLog(delay time.Duration) error
}

// penaltyFn1 and penaltyFn2 are functions with intersection in x=50 where penaltyFn2 is
// more steep.
func penaltyFn1(x int) int {
	return int(1000 * math.Pow(0.182*float64(x)+0.909, 0.9))
}

// penaltyFn2 and penaltyFn1 are functions with intersection in x=50 where penaltyFn2 is
// more steep.
func penaltyFn2(x int) int {
	return int(1000 * math.Pow(0.4*float64(x)-10, 0.9))
}

func penaltyFn3(x int) int {
	return int(1000 * math.Pow(0.8*float64(x), 0.9))
}

type Analyzer struct {
	backend Backend
	storage StatsStorage
	conf    *Conf
}

func (a *Analyzer) BotScore(req *http.Request) (float64, error) {
	return a.backend.BotScore(req)
}

func (a *Analyzer) Learn() error {
	return a.backend.Learn()
}

func (a *Analyzer) UserInducedResponseStatus(req *http.Request) services.ReqProperties {
	return services.ReqProperties{
		UserID:         -1,
		SessionID:      "",
		ProposedStatus: http.StatusOK,
		Error:          nil,
	}
}

func (a *Analyzer) CalcDelay(req *http.Request) (time.Duration, error) {
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	isBanned, err := a.storage.TestIPBan(net.ParseIP(ip))
	if err != nil {
		return 0, err
	}
	if isBanned {
		return ultraDuration, nil
	}
	botScore, err := a.backend.BotScore(req)
	if err == backend.ErrUnknownClient {
		log.Debug().Msgf(
			"client without telemetry (session %s, ip: %s)",
			sessionID, ip,
		)
		if sessionID != "" {
			// no telemetry - let's check client's request activity
			stats, err := a.storage.LoadStats(ip, sessionID, a.conf.WatchedTimeWindowSecs, true)
			if err != nil {
				return 0, err
			}
			if stats.Count > 50 {
				// user with a "long" session waits from ~8s to infinity
				// (e.g. 100 req. => ~14s, 500 req. => ~58s)
				return time.Duration(penaltyFn2(stats.Count)) * time.Millisecond, nil

			} else if stats.Count > 1 {
				// user with a "long" session and just a few requests
				// waits for ~1 to ~8 seconds
				return time.Duration(penaltyFn1(stats.Count)) * time.Millisecond, nil
			}
		}
		// no (valid) session (first request or a simple bot) => let's load stats for the whole IP
		stats, err := a.storage.LoadIPStats(ip, a.conf.WatchedTimeWindowSecs)
		log.Debug().Msgf(
			"loaded stats of whole IP (users without telemetry), num req: %d, latest: %v",
			stats.Count, stats.LastAccess)
		if err != nil {
			return 0, err
		}
		// user w
		return time.Duration(penaltyFn3(stats.Count)) * time.Millisecond, nil

	} else if err != nil {
		return 0, err

	} else {
		log.Debug().Msg("Client with telemetry...")
		// user with telemetry waits from 0 to ~6s
		return time.Duration(1000*2.5*botScore*2.5*botScore) * time.Millisecond, nil
	}
}

func (a *Analyzer) RegisterDelayLog(delay time.Duration) error {
	err := a.storage.RegisterDelayLog(delay)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register delay log")
	}
	return err
}

func NewAnalyzer(
	conf *Conf,
	telemetryConf *telemetry.Conf,
	monitoringConf *monitoring.ConnectionConf,
	db backend.TelemetryStorage,
	statsStorage StatsStorage,
) (*Analyzer, error) {
	switch telemetryConf.Analyzer {
	case "counting":
		return &Analyzer{
			conf:    conf,
			backend: counting.NewAnalyzer(db),
			storage: statsStorage,
		}, nil
	case "dumb":
		return &Analyzer{
			conf:    conf,
			backend: dumb.NewAnalyzer(db),
			storage: statsStorage,
		}, nil
	case "entropy":
		backend, err := entropy.NewAnalyzer(db, monitoringConf, telemetryConf)
		if err != nil {
			return nil, err
		}
		return &Analyzer{
			conf:    conf,
			backend: backend,
			storage: statsStorage,
		}, nil
	case "neural":
		backend, err := neural.NewAnalyzer(
			db,
			telemetryConf,
		)
		if err != nil {
			return nil, err
		}
		return &Analyzer{
			conf:    conf,
			backend: backend,
			storage: statsStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unknown analyzer backend %s", telemetryConf.Analyzer)
	}
}
