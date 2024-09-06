// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/monitoring"
	"apiguard/users"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/mail"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	alarmStatusFile                = "alarm-status.gob"
	dfltRecCountCleanupProbability = 0.5
	monitoringSendInterval         = time.Duration(30) * time.Second
	maxNumWatchedUserPrevReqs      = 5000
	numViolatedToReport            = 3
)

type reqCounterItem struct {
	Created time.Time
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
	ctx            *globctx.Context
	alarmConf      MailConf
	clients        *collections.ConcurrentMap[string, *serviceEntry] //save
	counter        chan guard.RequestInfo
	reports        []*AlarmReport //save
	location       *time.Location
	statusDataDir  string
	allowListUsers *collections.ConcurrentMap[string, []common.UserID]
	tDBWriter      *monitoring.TimescaleDBWriter
}

func (aticker *AlarmTicker) ServiceProps(servName string) *serviceEntry {
	return aticker.clients.Get(servName)
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

func (aticker *AlarmTicker) removeUsersWithNoRecentActivity() {
	aticker.clients.ForEach(func(k string, service *serviceEntry) {

		// find longest check interval:
		var maxInterval common.CheckInterval
		for chint := range service.limits {
			if chint > maxInterval {
				maxInterval = chint
			}
		}
		oldestTime := time.Now().In(aticker.location).Add(-time.Duration(maxInterval))

		service.ClientRequests.ForEach(func(userID common.UserID, limitInfo *UserLimitInfo) {
			mostRecent := limitInfo.Requests.Last()
			if mostRecent.Created.Before(oldestTime) {
				service.ClientRequests.Delete(userID)
			}
		})
	})
}

func (aticker *AlarmTicker) checkServiceUsage(service *serviceEntry, userID common.UserID) {
	counts := service.ClientRequests.Get(userID)
	for checkInterval, limit := range service.limits {
		numReq := counts.NumReqSince(time.Duration(checkInterval), aticker.location)
		log.Debug().Msgf("num requests since %s: %d", time.Duration(time.Duration(checkInterval)), numReq)
		if numReq > limit {
			counts.NumViolated[checkInterval]++
			log.Warn().
				Int("numViolated", counts.NumViolated[checkInterval]).
				Int("threshold", numViolatedToReport).
				Int("userId", int(userID)).
				Str("service", service.Service).
				Msgf("detected high activity [yet unreported]")
			if counts.NumViolated[checkInterval] >= numViolatedToReport {
				newReport := NewAlarmReport(
					guard.RequestInfo{
						Created:     time.Now().In(aticker.location),
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
					aticker.ctx.CNCDB, aticker.ctx.UserTableProps))
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
				go aticker.sendReport(service, newReport, userID, numReq)
				log.Warn().Msgf("detected high activity for service %s and user %d", service.Service, userID)
				counts.NumViolated[checkInterval] = 0
			}
		}
	}
}

func (aticker *AlarmTicker) loadAllowList() {
	aticker.allowListUsers = collections.NewConcurrentMap[string, []common.UserID]()
	var total int
	aticker.clients.ForEach(func(serviceID string, se *serviceEntry) {
		v, err := guard.GetAllowlistUsers(aticker.ctx.CNCDB, serviceID)
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

func (aticker *AlarmTicker) reqIsIgnorable(reqInfo guard.RequestInfo) bool {
	alist := aticker.allowListUsers.Get(reqInfo.Service)
	return collections.SliceContains(alist, reqInfo.UserID) || !reqInfo.UserID.IsValid()
}

func (aticker *AlarmTicker) Shutdown(ctx context.Context) error {
	saveDone := make(chan bool)
	var err error
	go func() {
		err = SaveState(aticker)
		close(saveDone)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-saveDone:
		return err
	}
}

func (aticker *AlarmTicker) Run(reloadChan <-chan bool) {
	aticker.loadAllowList()
	for {
		select {
		case <-aticker.ctx.Done():
			log.Debug().Msg("AlarmTicker got shutdown signal")
			return
		case reload := <-reloadChan:
			if reload {
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
						&UserLimitInfo{
							Requests:    collections.NewCircularList[reqCounterItem](maxNumWatchedUserPrevReqs),
							NumViolated: make(map[common.CheckInterval]int),
						},
					)
				}
				entry.ClientRequests.
					Get(reqInfo.UserID).
					Requests.
					Append(reqCounterItem{Created: reqInfo.Created})
			}
			go func(item guard.RequestInfo) {
				aticker.checkServiceUsage(
					aticker.clients.Get(item.Service),
					item.UserID,
				)
			}(reqInfo)
			// from time to time, remove users with no recent activity
			if rand.Float64() < aticker.clients.Get(reqInfo.Service).Conf.RecCounterCleanupProbability {
				go aticker.removeUsersWithNoRecentActivity()
			}
		}
	}
}

// Register initializes the AlarmTicker instance to watch for number and ratio
// of incoming requests for a specific service. It returns a channel which is
// expected to be used by a correspoding service proxy to log incoming requests.
func (aticker *AlarmTicker) Register(service string, conf AlarmConf, limits []Limit) chan<- guard.RequestInfo {
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
					aticker.ctx.CNCDB,
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
	ctx *globctx.Context,
	loc *time.Location,
	alarmConf MailConf,
	statusDataDir string,
) *AlarmTicker {
	return &AlarmTicker{
		ctx:            ctx,
		clients:        collections.NewConcurrentMap[string, *serviceEntry](),
		counter:        make(chan guard.RequestInfo, 1000),
		location:       loc,
		alarmConf:      alarmConf,
		statusDataDir:  statusDataDir,
		allowListUsers: collections.NewConcurrentMap[string, []common.UserID](),
		tDBWriter:      ctx.TimescaleDBWriter,
	}
}
