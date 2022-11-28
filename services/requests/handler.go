// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package requests

import (
	"apiguard/services"
	"apiguard/storage"
	"net/http"
	"strconv"
)

type Actions struct {
	db *storage.MySQLAdapter
}

func (a *Actions) List(w http.ResponseWriter, req *http.Request) {
	limitArg := req.URL.Query().Get("limit")
	limit := 50
	if limitArg != "" {
		var err error
		limit, err = strconv.Atoi(limitArg)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 422)
			return
		}
	}

	maxAgeArg := req.URL.Query().Get("maxAge")
	maxAge := 3600 * 24
	if maxAgeArg != "" {
		var err error
		maxAge, err = strconv.Atoi(maxAgeArg)
		if err != nil {
			services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 422)
			return
		}
	}

	items, err := a.db.LoadStatsList(limit, maxAge)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
	}
	services.WriteJSONResponse(w, items)
}

func NewActions(db *storage.MySQLAdapter) *Actions {
	return &Actions{db: db}
}
