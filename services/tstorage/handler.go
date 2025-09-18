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
	db telemetry.Storage
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

func NewActions(db telemetry.Storage) *Actions {
	return &Actions{db: db}
}
