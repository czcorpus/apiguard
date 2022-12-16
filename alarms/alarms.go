// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/alarms/mail"
	"apiguard/cncdb"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/czcorpus/uniresp"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type serviceEntry struct {
	conf           AlarmConf
	limits         []Limit
	service        string
	clientRequests map[int]int // userID -> num requests
}

type RequestInfo struct {
	Service     string `json:"service"`
	NumRequests int    `json:"numRequests"`
	UserID      int    `json:"userId"`
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

type AlarmTicker struct {
	db             *sql.DB
	alarmConf      MailConf
	clients        map[string]*serviceEntry
	servicesLock   sync.Mutex
	ticker         *time.Ticker
	counter        chan RequestInfo
	reports        []*AlarmReport
	location       *time.Location
	userTableProps cncdb.UserTableProps
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

func (aticker *AlarmTicker) checkService(entry *serviceEntry, name string, unixTime int64) {
	for _, limit := range entry.limits {
		if unixTime%int64(limit.ReqCheckingIntervalSecs) == 0 {
			for userID, numReq := range entry.clientRequests {
				if numReq > limit.ReqPerTimeThreshold {
					newReport := NewAlarmReport(
						RequestInfo{
							Service:     entry.service,
							NumRequests: numReq,
							UserID:      userID,
						},
						entry.conf,
						limit,
						aticker.location,
					)
					err := newReport.AttachUserInfo(cncdb.NewUsersTable(
						aticker.db, aticker.userTableProps))
					if err != nil {
						newReport.UserInfo = &cncdb.User{
							ID:          -1,
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

					go func() {
						client, err := uniresp.DialSmtpServer(
							aticker.alarmConf.SMTPServer,
							aticker.alarmConf.SmtpUsername,
							aticker.alarmConf.SmtpPassword,
						)
						if err != nil {
							log.Error().Err(err).Msg("failed to send alarm e-mail")
							return
						}
						defer client.Close()
						if err != nil {
							log.Error().Err(err).Msg("failed to send alarm e-mail")
							return
						}

						for _, recipient := range entry.conf.Recipients {
							log.Debug().Msgf("about to send a notification e-mail to %s", recipient)
							page := aticker.createConfirmationPageURL(newReport, recipient)
							err := mail.SendNotification(
								client,
								aticker.location,
								aticker.alarmConf.Sender,
								[]string{recipient},
								fmt.Sprintf(
									"CNC APIGuard - překročení přístupů k API o %01.1f%% u služby '%s'",
									newReport.ExceedPercent(), entry.service,
								),
								fmt.Sprintf(
									"Byl detekován velký počet API dotazů na službu '%s' od uživatele ID %d: %d za posledních %d sekund.<br /> "+
										"Max. povolený limit pro tuto službu je %d dotazů za %d sekund.",
									entry.service, userID, numReq, newReport.Rules.ReqCheckingIntervalSecs,
									newReport.Rules.ReqPerTimeThreshold,
									newReport.Rules.ReqCheckingIntervalSecs,
								),
								fmt.Sprintf(
									"Detaily získáte a hlášení potvrdíte kliknutím na odkaz:<br /> <a href=\"%s\">%s</a>",
									page, page,
								),
							)
							if err != nil {
								log.Error().
									Err(err).
									Msgf("failed to send a notification e-mail to %s", recipient)
							}
						}
					}()
					entry.clientRequests[userID] = 0
					log.Warn().Msgf("detected high activity for service %s and user %d", entry.service, userID)
				}
			}
		}
	}
}

func (aticker *AlarmTicker) Run(quitChan <-chan os.Signal) {
	go func() {
		for item := range aticker.counter {
			if entry, ok := aticker.clients[item.Service]; ok {
				entry.clientRequests[item.UserID] += item.NumRequests
				aticker.clients[item.Service] = entry
			}
		}
	}()
	aticker.ticker = time.NewTicker(time.Second)
	for {
		select {
		case v := <-aticker.ticker.C:
			for name, service := range aticker.clients {
				aticker.checkService(service, name, v.UnixMilli()/1000)
			}

		case <-quitChan:
			aticker.ticker.Stop()
		}
	}
}

func (aticker *AlarmTicker) Register(service string, conf AlarmConf, limits []Limit) chan<- RequestInfo {
	aticker.servicesLock.Lock()
	aticker.clients[service] = &serviceEntry{
		service:        service,
		conf:           conf,
		limits:         limits,
		clientRequests: make(map[int]int),
	}
	aticker.servicesLock.Unlock()
	log.Info().Msgf("Registered alarm for %s", service)
	return aticker.counter
}

func (aticker *AlarmTicker) HandleReportListAction(w http.ResponseWriter, req *http.Request) {

	uniresp.WriteJSONResponse(w, map[string]any{"reports": aticker.reports})

}

func (aticker *AlarmTicker) HandleReviewAction(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	alarmID := vars["alarmID"]

	var qry handleReviewPayload
	err := json.NewDecoder(req.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			w, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}

	for _, report := range aticker.reports {
		if report.ReviewCode == alarmID {
			err := report.ConfirmReviewViaEmail(alarmID, qry.Reviewer)
			if err == ErrConfirmationKeyNotFound {
				uniresp.WriteJSONErrorResponse(
					w,
					uniresp.NewActionErrorFrom(err),
					http.StatusNotFound,
				)
				return
			}
			if err == ErrMissingReviewerIdentification {
				uniresp.WriteJSONErrorResponse(
					w,
					uniresp.NewActionErrorFrom(err),
					http.StatusBadRequest,
				)
				return
			}
			if err != nil {
				uniresp.WriteJSONErrorResponse(
					w,
					uniresp.NewActionErrorFrom(err),
					http.StatusInternalServerError,
				)
				return
			}

			var banID int64
			if qry.BanHours > 0 {
				now := time.Now().In(aticker.location)
				banID, err = cncdb.BanUser(
					aticker.db,
					aticker.location,
					report.RequestInfo.UserID,
					&alarmID,
					now,
					now.Add(time.Duration(qry.BanHours)*time.Hour),
				)
				if err != nil {
					uniresp.WriteJSONErrorResponse(
						w,
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
			uniresp.WriteJSONResponse(w, ans)
			return
		}
	}
	uniresp.WriteJSONErrorResponse(
		w,
		uniresp.NewActionError("confirmation key not found"),
		http.StatusNotFound,
	)
}

func NewAlarmTicker(
	db *sql.DB,
	loc *time.Location,
	alarmConf MailConf,
	userTableProps cncdb.UserTableProps,
) *AlarmTicker {
	return &AlarmTicker{
		db:             db,
		clients:        make(map[string]*serviceEntry),
		counter:        make(chan RequestInfo, 1000),
		location:       loc,
		alarmConf:      alarmConf,
		userTableProps: userTableProps,
	}
}
