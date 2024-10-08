// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package tstorage

import (
	"apiguard/services/logging"
	"apiguard/telemetry"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type actionRecord struct {
	ActionName  string `json:"actionName"`
	IsMobile    bool   `json:"isMobile"`
	IsSubquery  bool   `json:"isSubquery"`
	TileName    string `json:"tileName"`
	TimestampMS int64  `json:"timestamp"`
}

type payload struct {
	Telemetry []*actionRecord `json:"telemetry"`
}

type Actions struct {
	db *telemetry.DelayStats
}

func (a *Actions) Store(ctx *gin.Context) {
	rawPayload, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	var payloadTmp payload
	err = json.Unmarshal(rawPayload, &payloadTmp)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	ip, sessionID := logging.ExtractRequestIdentifiers(ctx.Request)
	payload := telemetry.Payload{
		Telemetry: make([]*telemetry.ActionRecord, len(payloadTmp.Telemetry)),
	}
	for i, item := range payloadTmp.Telemetry {
		payload.Telemetry[i] = &telemetry.ActionRecord{
			Client: telemetry.Client{
				SessionID: sessionID,
				IP:        ip,
			},
			ActionName: item.ActionName,
			IsMobile:   item.IsMobile,
			IsSubquery: item.IsSubquery,
			TileName:   item.TileName,
			Created:    time.UnixMilli(item.TimestampMS),
		}
	}

	transact, err := a.db.StartTx()
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	err = a.db.InsertTelemetry(transact, payload)
	if err != nil {
		log.Error().Err(err).Msg("")
		a.db.RollbackTx(transact)
		return
	}

	err = a.db.CommitTx(transact)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

}

func NewActions(db *telemetry.DelayStats) *Actions {
	return &Actions{db: db}
}
