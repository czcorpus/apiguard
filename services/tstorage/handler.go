// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package tstorage

import (
	"apiguard/logging"
	"apiguard/services"
	"apiguard/storage"
	"apiguard/telemetry"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

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
	db *storage.MySQLAdapter
}

func (a *Actions) Store(w http.ResponseWriter, req *http.Request) {
	rawPayload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		services.WriteJSONErrorResponse(
			w, services.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	var payloadTmp payload
	err = json.Unmarshal(rawPayload, &payloadTmp)
	if err != nil {
		services.WriteJSONErrorResponse(
			w, services.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
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

func NewActions(db *storage.MySQLAdapter) *Actions {
	return &Actions{db: db}
}
