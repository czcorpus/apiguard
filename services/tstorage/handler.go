// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package tstorage

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"wum/logging"
	"wum/services"
	"wum/storage"
	"wum/telemetry"
)

type Actions struct {
	db *storage.MySQLAdapter
}

func (a *Actions) Store(w http.ResponseWriter, req *http.Request) {
	rawPayload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		services.WriteJSONErrorResponse(
			w, services.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	var payload telemetry.Payload
	err = json.Unmarshal(rawPayload, &payload)
	if err != nil {
		services.WriteJSONErrorResponse(
			w, services.NewActionError(err.Error()), http.StatusInternalServerError)
	}
	ip, sessionID := logging.ExtractRequestIdentifiers(req)
	for _, item := range payload.Telemetry {
		item.ClientIP = ip
		item.SessionID = sessionID
	}

	transact, err := a.db.StartTx()
	if err != nil {
		log.Print("ERROR: ", err)
		return
	}

	err = a.db.InsertTelemetry(transact, payload)
	if err != nil {
		log.Print("ERROR: ", err)
		a.db.RollbackTx(transact)
		return
	}

	err = a.db.CommitTx(transact)
	if err != nil {
		log.Print("ERROR: ", err)
	}

}

func NewActions(db *storage.MySQLAdapter) *Actions {
	return &Actions{db: db}
}
