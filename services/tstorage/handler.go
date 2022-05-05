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

	log.Print("DEBUG: got telemetry payload: ", payload)

}

func NewActions(db *storage.MySQLAdapter) *Actions {
	return &Actions{db: db}
}
