// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package requests

import (
	"apiguard/cncdb"
	"net/http"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

type Actions struct {
	db *cncdb.DelayStats
}

func (a *Actions) List(w http.ResponseWriter, req *http.Request) {
	limitArg := req.URL.Query().Get("limit")
	limit := 50
	if limitArg != "" {
		var err error
		limit, err = strconv.Atoi(limitArg)
		if err != nil {
			uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 422)
			return
		}
	}

	maxAgeArg := req.URL.Query().Get("maxAge")
	maxAge := 3600 * 24
	if maxAgeArg != "" {
		var err error
		maxAge, err = strconv.Atoi(maxAgeArg)
		if err != nil {
			uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 422)
			return
		}
	}

	items, err := a.db.LoadStatsList(limit, maxAge)
	if err != nil {
		uniresp.WriteJSONErrorResponse(w, uniresp.NewActionError(err.Error()), 500)
	}
	uniresp.WriteJSONResponse(w, items)
}

func NewActions(db *cncdb.DelayStats) *Actions {
	return &Actions{db: db}
}
