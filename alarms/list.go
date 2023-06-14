// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"net/http"
	"sort"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type cleanActionResponse struct {
	NumDeleted   int `json:"numDeleted"`
	NumRemaining int `json:"numRemaining"`
}

func (aticker *AlarmTicker) HandleListAction(ctx *gin.Context) {

	recordsCopy := make([]AlarmReport, len(aticker.reports))
	for i, report := range aticker.reports {
		recordsCopy[i] = *report
	}
	sort.Slice(recordsCopy, func(i, j int) bool {
		return recordsCopy[i].Created.Before(recordsCopy[j].Created)
	})
	uniresp.WriteJSONResponse(ctx.Writer, recordsCopy)
}

func (aticker *AlarmTicker) HandleCleanAction(ctx *gin.Context) {
	err := unireq.CheckSuperfluousURLArgs(ctx.Request, []string{"maxAge", "alsoNonReviewed"})
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusBadRequest)
	}
	maxAge, err := datetime.ParseDuration(ctx.Request.URL.Query().Get("maxAge"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusUnprocessableEntity,
		)
		return
	}
	if maxAge == 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("maxAge argument must be greater than zero"),
			http.StatusBadRequest,
		)
	}

	var includeNonReviewed bool
	if ctx.Request.URL.Query().Get("alsoNonReviewed") == "1" {
		includeNonReviewed = true
	}

	remainReports := make([]*AlarmReport, 0, len(aticker.reports))
	now := time.Now().In(aticker.location)
	for _, report := range aticker.reports {
		if report.Created.After(now.Add(-maxAge)) || (!includeNonReviewed && !report.IsReviewed()) {
			remainReports = append(remainReports, report)
		}
	}
	sort.Slice(remainReports, func(i, j int) bool {
		return remainReports[i].Created.Before(remainReports[j].Created)
	})
	var resp cleanActionResponse
	resp.NumDeleted = len(aticker.reports) - len(remainReports)
	resp.NumRemaining = len(remainReports)
	aticker.reports = remainReports
	uniresp.WriteJSONResponse(ctx.Writer, resp)
}
