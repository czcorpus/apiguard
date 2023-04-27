// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"apiguard/common"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/rs/zerolog/log"
)

type ReqProperties struct {
	UserID         common.UserID
	SessionID      string
	ProposedStatus int
	Error          error
}

func (rp ReqProperties) ForbidsAccess() bool {
	return rp.ProposedStatus >= 400 && rp.ProposedStatus < 500
}

type ReqAnalyzer interface {
	CalcDelay(req *http.Request) (time.Duration, error)
	LogAppliedDelay(respDelay time.Duration) error
	UserInducedResponseStatus(req *http.Request, serviceName string) ReqProperties
}

func RestrictResponseTime(w http.ResponseWriter, req *http.Request, readTimeoutSecs int, analyzer ReqAnalyzer) error {
	respDelay, err := analyzer.CalcDelay(req)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			w,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		log.Error().Err(err).Msg("failed to analyze client")
		return err
	}
	log.Debug().Msgf("Client is going to wait for %v", respDelay)
	if respDelay.Seconds() >= float64(readTimeoutSecs) {
		uniresp.WriteJSONErrorResponse(
			w,
			uniresp.NewActionError("service overloaded"),
			http.StatusServiceUnavailable,
		)
		return err
	}
	go analyzer.LogAppliedDelay(respDelay)
	time.Sleep(respDelay)
	return nil
}
