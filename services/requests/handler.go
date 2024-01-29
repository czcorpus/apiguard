// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package requests

import (
	"apiguard/cnc/guard"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	db *guard.DelayStats
}

func (a *Actions) List(ctx *gin.Context) {
	limitArg := ctx.Request.URL.Query().Get("limit")
	limit := 50
	if limitArg != "" {
		var err error
		limit, err = strconv.Atoi(limitArg)
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 422)
			return
		}
	}

	maxAgeArg := ctx.Request.URL.Query().Get("maxAge")
	maxAge := 3600 * 24
	if maxAgeArg != "" {
		var err error
		maxAge, err = strconv.Atoi(maxAgeArg)
		if err != nil {
			uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 422)
			return
		}
	}

	items, err := a.db.LoadStatsList(limit, maxAge)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(err.Error()), 500)
	}
	uniresp.WriteJSONResponse(ctx.Writer, items)
}

func NewActions(db *guard.DelayStats) *Actions {
	return &Actions{db: db}
}
