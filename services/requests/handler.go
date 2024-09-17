// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package requests

import (
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/monitoring"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	topNumActiveClients = 20
)

type userTotal struct {
	UserIdent  string    `json:"userIdent"`
	NumReq     int       `json:"numReq"`
	Exceeding  any       `json:"exceeding"`
	LastReport time.Time `json:"lastReport,omitempty"`
}

type activityResponse struct {
	MostActiveUsers   []userTotal `json:"mostActiveUsers"`
	TotalWatchedUsers int         `json:"totalWatchedUsers"`
}

type Actions struct {
	gctx  *globctx.Context
	db    *guard.DelayStats
	alarm *monitoring.AlarmTicker
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

func (a *Actions) Activity(ctx *gin.Context) {
	serviceID := ctx.Param("serviceID")
	since, err := datetime.ParseDuration(ctx.Query("ago"))
	if since == 0 {
		newUrl := *ctx.Request.URL
		nuQuery := newUrl.Query()
		nuQuery.Set("ago", "1h")
		newUrl.RawQuery = nuQuery.Encode()
		ctx.Redirect(http.StatusSeeOther, newUrl.String())
		return
	}
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}

	servProps := a.alarm.ServiceProps(serviceID)
	if servProps == nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("service %s not registered in AlarmTicker", serviceID),
			http.StatusNotFound,
		)
		return
	}
	t0 := time.Now().In(a.gctx.TimezoneLocation)
	reqCounts := make([]userTotal, 0, 100)
	servProps.ClientRequests.ForEach(func(k string, v *monitoring.UserActivity, ok bool) {
		if !ok {
			return
		}
		v.NumReqAboveLimit.Touch(t0)
		reqCounts = append(
			reqCounts,
			userTotal{
				UserIdent:  k,
				NumReq:     v.NumReqSince(since, a.gctx.TimezoneLocation),
				Exceeding:  v.NumReqAboveLimit,
				LastReport: v.LastReportAt,
			},
		)
	})
	slices.SortStableFunc(reqCounts, func(a, b userTotal) int {
		if a.NumReq < b.NumReq {
			return -1

		} else if a.NumReq > b.NumReq {
			return 1
		}
		return 0
	})
	if len(reqCounts) > topNumActiveClients {
		reqCounts = reqCounts[:topNumActiveClients]
	}
	ans := activityResponse{
		MostActiveUsers:   reqCounts,
		TotalWatchedUsers: servProps.ClientRequests.Len(),
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewActions(
	gctx *globctx.Context,
	db *guard.DelayStats,
	alarm *monitoring.AlarmTicker,
) *Actions {
	return &Actions{
		gctx:  gctx,
		db:    db,
		alarm: alarm,
	}
}
