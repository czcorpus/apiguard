// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/users"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/influx"
	"github.com/czcorpus/cnc-gokit/mail"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	alarmStatusFile                = "alarm-status.gob"
	dfltRecCountCleanupProbability = 0.5
	monitoringSendInterval         = time.Duration(30) * time.Second
)

type reqCounterItem struct {
	Created time.Time
}

type RequestInfo struct {
	Service     string        `json:"service"`
	NumRequests int           `json:"numRequests"`
	UserID      common.UserID `json:"userId"`
}

type handleReviewPayload struct {
	Reviewer string `json:"reviewer"`
	BanHours int    `json:"banHours"`
}

type handleReviewResponse struct {
	Confirmed bool         `json:"confirmed"`
	Report    *AlarmReport `json:"report"`
	BanID     int64        `json:"banId,omitempty"`
}

// AlarmTicker monitors the 'counter' channel for incoming
// RequestInfo values, accumulating the request count
// for each user. Periodically, it checks these request
// counts against preset limits. If a user's request count
// surpasses the set limit, AlarmTicker notifies administrators
// and suggests a ban (via an e-mail sent to the administrators).
//
// It also listens for os signals and in case of exit it
// serializes runtime values (e.g. the current counts).
type AlarmTicker struct {
	db             *sql.DB
	alarmConf      MailConf
	clients        *collections.ConcurrentMap[string, *serviceEntry] //save
	counter        chan RequestInfo
	reports        []*AlarmReport //save
	location       *time.Location
	userTableProps users.UserTableProps
	statusDataDir  string
	allowListUsers *collections.ConcurrentMap[string, []common.UserID]
	monitoring     *influx.RecordWriter[alarmStatus]
}

func (aticker *AlarmTicker) createConfirmationURL(report *AlarmReport, reviewer string) string {
	return fmt.Sprintf(
		"%s/alarm/%s/confirmation?reviewer=%s",
		aticker.alarmConf.ConfirmationBaseURL, report.ReviewCode, reviewer,
	)
}

func (aticker *AlarmTicker) createConfirmationPageURL(report *AlarmReport, reviewer string) string {
	return fmt.Sprintf(
		"%s/alarm-confirmation?id=%s&reviewer=%s",
		aticker.alarmConf.ConfirmationBaseURL, report.ReviewCode, reviewer,
	)
}

func (aticker *AlarmTicker) sendReport(
	service *serviceEntry,
	report *AlarmReport,
	userID common.UserID,
	numReq int,
) {
	for _, recipient := range service.Conf.Recipients {
		log.Debug().Msgf("about to send a notification e-mail to %s", recipient)
		page := aticker.createConfirmationPageURL(report, recipient)
		msg := mail.Notification{
			Subject: fmt.Sprintf(
				"CNC APIGuard - překročení přístupů k API o %01.1f%% u služby '%s'",
				report.ExceedPercent(), service.Service,
			),
			Paragraphs: []string{
				fmt.Sprintf(
					"Byl detekován velký počet API dotazů na službu '%s' od uživatele ID %d: %d za posledních %d sekund.<br /> "+
						"Limit, který byl překročen, je: %d dotazů za %s.",
					service.Service, userID, numReq, report.Rules.ReqCheckingIntervalSecs,
					report.Rules.ReqPerTimeThreshold,
					datetime.DurationToHMS(report.Rules.ReqCheckingInterval()),
				),
				fmt.Sprintf(
					"Detaily získáte a hlášení potvrdíte kliknutím na odkaz:<br /> <a href=\"%s\">%s</a>",
					page, page,
				),
			},
		}
		msgCnf := aticker.alarmConf.WithRecipients(recipient)
		err := mail.SendNotification(
			&msgCnf,
			aticker.location,
			msg,
		)
		if err != nil {
			log.Error().
				Err(err).
				Msgf("failed to send a notification e-mail to %s", recipient)
		}
	}
}

func (aticker *AlarmTicker) checkServiceUsage(service *serviceEntry, userID common.UserID) {
	counts := service.ClientRequests.Get(userID)
	for checkInterval, limit := range service.limits {
		numReq := counts.NumReqSince(time.Duration(checkInterval), aticker.location)
		log.Debug().Msgf("num requests since %s: %d", time.Duration(time.Duration(checkInterval)), numReq)
		if numReq > limit && !counts.Reported[checkInterval] {
			newReport := NewAlarmReport(
				RequestInfo{
					Service:     service.Service,
					NumRequests: numReq,
					UserID:      userID,
				},
				service.Conf,
				Limit{
					ReqCheckingIntervalSecs: checkInterval.ToSeconds(),
					ReqPerTimeThreshold:     service.limits[checkInterval],
				},
				aticker.location,
			)
			if !newReport.IsSignificantlyExceeding() {
				continue
			}

			err := newReport.AttachUserInfo(users.NewUsersTable(
				aticker.db, aticker.userTableProps))
			if err != nil {
				newReport.UserInfo = &users.User{
					ID:          common.InvalidUserID,
					Username:    "invalid",
					FirstName:   "-",
					LastName:    "-",
					Affiliation: "-",
				}
				log.Error().
					Err(err).
					Str("reportId", newReport.ReviewCode).
					Msg("failed to attach user info to a report")
			}
			aticker.reports = append(aticker.reports, newReport)
			counts.Reported[checkInterval] = true

			go aticker.sendReport(service, newReport, userID, numReq)

			log.Warn().Msgf("detected high activity for service %s and user %d", service.Service, userID)
		}
	}
}

func (aticker *AlarmTicker) loadAllowList() {
	aticker.allowListUsers = collections.NewConcurrentMap[string, []common.UserID]()
	var total int
	aticker.clients.ForEach(func(serviceID string, se *serviceEntry) {
		v, err := guard.GetAllowlistUsers(aticker.db, serviceID)
		if err != nil {
			log.Error().
				Err(err).
				Str("service", serviceID).
				Msg("Failed to reload user allow list")
			return
		}
		aticker.allowListUsers.Set(serviceID, v)
		total += len(v)
	})
	log.Info().
		Int("itemsLoaded", total).
		Msg("Reloaded user allow lists for all services.")
}

func (aticker *AlarmTicker) clearOldRecords(service *serviceEntry) {
	// find longest check interval:
	var maxInterval common.CheckInterval
	for chint := range service.limits {
		if chint > maxInterval {
			maxInterval = chint
		}
	}
	oldestDate := time.Now().In(aticker.location).Add(-time.Duration(maxInterval))
	var oldNumReq, newNumReq int
	service.ClientRequests.ForEach(func(k common.UserID, data *userLimitInfo) {
		oldNumReq += len(data.Requests)
		keep := make([]reqCounterItem, 0, 1000)
		for _, req := range data.Requests {
			if req.Created.After(oldestDate) {
				keep = append(keep, req)
			}
		}
		data.Requests = keep
		newNumReq += len(keep)
	})
	cleanupSummary := "Data size unchanged"
	if newNumReq < oldNumReq {
		cleanupSummary = fmt.Sprintf("Reduced number of records from %d to %d", oldNumReq, newNumReq)
	}
	log.Info().
		Int("numOldRec", oldNumReq).
		Int("numNewRec", newNumReq).
		Msgf("Performed old requests cleanup. %s", cleanupSummary)
}

func (aticker *AlarmTicker) reqIsIgnorable(reqInfo RequestInfo) bool {
	alist := aticker.allowListUsers.Get(reqInfo.Service)
	return collections.SliceContains(alist, reqInfo.UserID) || !reqInfo.UserID.IsValid()
}

func (aticker *AlarmTicker) Run(quitChan <-chan os.Signal) {
	aticker.loadAllowList()
	for {
		select {
		case signal := <-quitChan:
			if signal == syscall.SIGHUP {
				aticker.loadAllowList()
			}
		case reqInfo := <-aticker.counter:
			if aticker.reqIsIgnorable(reqInfo) {
				break
			}
			if entry, ok := aticker.clients.GetWithTest(reqInfo.Service); ok {
				if !entry.ClientRequests.HasKey(reqInfo.UserID) {
					entry.ClientRequests.Set(
						reqInfo.UserID,
						&userLimitInfo{
							Requests: make([]reqCounterItem, 0, 1000),
							Reported: make(map[common.CheckInterval]bool),
						},
					)
				}
				entry.ClientRequests.Get(reqInfo.UserID).Requests = append(
					entry.ClientRequests.Get(reqInfo.UserID).Requests,
					reqCounterItem{
						Created: time.Now().In(aticker.location),
					},
				)
			}
			go func(item RequestInfo) {
				aticker.checkServiceUsage(
					aticker.clients.Get(item.Service),
					item.UserID,
				)
			}(reqInfo)
			// from time to time, remove older records
			if rand.Float64() < aticker.clients.Get(reqInfo.Service).Conf.RecCounterCleanupProbability {
				go aticker.clearOldRecords(aticker.clients.Get(reqInfo.Service))
			}
		}
	}
}

func (aticker *AlarmTicker) Register(service string, conf AlarmConf, limits []Limit) chan<- RequestInfo {
	if conf.RecCounterCleanupProbability == 0 {
		log.Warn().Msgf(
			"Service's recCounterCleanupProbability not set. Using default %0.2f",
			dfltRecCountCleanupProbability,
		)
		conf.RecCounterCleanupProbability = dfltRecCountCleanupProbability
	}
	sEntry := &serviceEntry{
		Service:        service,
		Conf:           conf,
		limits:         make(map[common.CheckInterval]int),
		ClientRequests: NewClientRequests(),
	}
	for _, limit := range limits {
		sEntry.limits[common.CheckInterval(limit.ReqCheckingInterval())] = limit.ReqPerTimeThreshold
	}
	aticker.clients.Set(service, sEntry)
	log.Info().Msgf("Registered alarm for %s", service)
	return aticker.counter
}

func (aticker *AlarmTicker) HandleReportListAction(ctx *gin.Context) {

	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"reports": aticker.reports})

}

func (aticker *AlarmTicker) HandleReviewAction(ctx *gin.Context) {
	alarmID := ctx.Param("alarmId")

	var qry handleReviewPayload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}

	for _, report := range aticker.reports {
		if report.ReviewCode == alarmID {
			err := report.ConfirmReviewViaEmail(alarmID, qry.Reviewer)
			if err == ErrConfirmationKeyNotFound {
				uniresp.WriteJSONErrorResponse(
					ctx.Writer,
					uniresp.NewActionErrorFrom(err),
					http.StatusNotFound,
				)
				return
			}
			if err == ErrMissingReviewerIdentification {
				uniresp.WriteJSONErrorResponse(
					ctx.Writer,
					uniresp.NewActionErrorFrom(err),
					http.StatusBadRequest,
				)
				return
			}
			if err != nil {
				uniresp.WriteJSONErrorResponse(
					ctx.Writer,
					uniresp.NewActionErrorFrom(err),
					http.StatusInternalServerError,
				)
				return
			}

			var banID int64
			if qry.BanHours > 0 {
				now := time.Now().In(aticker.location)
				banID, err = guard.BanUser(
					aticker.db,
					aticker.location,
					report.RequestInfo.UserID,
					&alarmID,
					now,
					now.Add(time.Duration(qry.BanHours)*time.Hour),
				)
				if err != nil {
					uniresp.WriteJSONErrorResponse(
						ctx.Writer,
						uniresp.NewActionErrorFrom(err),
						http.StatusInternalServerError,
					)
					return
				}
			}
			ans := handleReviewResponse{
				Confirmed: true,
				Report:    report,
				BanID:     banID,
			}
			uniresp.WriteJSONResponse(ctx.Writer, ans)
			return
		}
	}
	uniresp.WriteJSONErrorResponse(
		ctx.Writer,
		uniresp.NewActionError("confirmation key not found"),
		http.StatusNotFound,
	)
}

func NewAlarmTicker(
	ctx *ctx.GlobalContext,
	loc *time.Location,
	alarmConf MailConf,
	userTableProps users.UserTableProps,
	statusDataDir string,
) *AlarmTicker {
	return &AlarmTicker{
		db:             ctx.CNCDB,
		clients:        collections.NewConcurrentMap[string, *serviceEntry](),
		counter:        make(chan RequestInfo, 1000),
		location:       loc,
		alarmConf:      alarmConf,
		userTableProps: userTableProps,
		statusDataDir:  statusDataDir,
		allowListUsers: collections.NewConcurrentMap[string, []common.UserID](),
		monitoring:     influx.NewRecordWriter[alarmStatus](ctx.InfluxDB),
	}
}
