// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package users

import (
	"apiguard/cncdb"
	"apiguard/common"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	conf     *Conf
	location *time.Location
	cncDB    *sql.DB
}

func (a *Actions) BanInfo(ctx *gin.Context) {
	userID, err := strconv.Atoi(ctx.Param("userID"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	ban, err := cncdb.MostRecentActiveBan(a.cncDB, a.location, userID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ban)
}

func (a *Actions) SetBan(ctx *gin.Context) {
	userID, err := common.Str2UserID(ctx.Param("userID"))
	days := ctx.Request.URL.Query().Get("days")
	hours := ctx.Request.URL.Query().Get("hours")
	var banLen time.Duration
	if days != "" {
		ndays, err := strconv.Atoi(days)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}
		banLen += time.Duration(ndays) * time.Hour * 24
	}
	if hours != "" {
		nhours, err := strconv.Atoi(hours)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}
		banLen += time.Duration(nhours) * time.Hour
	}
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	now := time.Now().In(a.location)
	newID, err := cncdb.BanUser(a.cncDB, a.location, userID, nil, now, now.Add(banLen))
	if err == cncdb.ErrorUserAlreadyBannned {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusUnprocessableEntity,
		)

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"banId": newID})
}

func (a *Actions) DisableBan(ctx *gin.Context) {
	userID, err := strconv.Atoi(ctx.Param("userID"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	numBans, err := cncdb.UnbanUser(a.cncDB, a.location, userID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"bansRemoved": numBans})

}

func NewActions(conf *Conf, cncDB *sql.DB, loc *time.Location) *Actions {
	return &Actions{
		conf:     conf,
		cncDB:    cncDB,
		location: loc,
	}
}
