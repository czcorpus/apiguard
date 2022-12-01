// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/alarms/mail"
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
	conf           Conf
	service        string
	clientRequests map[int]int // userID -> num requests
}

type RequestInfo struct {
	Service     string `json:"service"`
	NumRequests int    `json:"numRequests"`
	UserID      int    `json:"userId"`
}

type AlarmTicker struct {
	alarmConf      MailConf
	clients        map[string]*serviceEntry
	servicesLock   sync.Mutex
	ticker         *time.Ticker
	counter        chan RequestInfo
	reports        []*AlarmReport
	location       *time.Location
	usersTableName string
}

func (aticker *AlarmTicker) checkService(entry *serviceEntry, name string, unixTime int64) {
	if unixTime%int64(entry.conf.ReqCheckingIntervalSecs) == 0 {
		for userID, numReq := range entry.clientRequests {
			if numReq > entry.conf.ReqPerTimeThreshold {
				newReport := NewAlarmReport(
					RequestInfo{
						Service:     entry.service,
						NumRequests: numReq,
						UserID:      userID,
					},
					entry.conf,
					aticker.location,
				)
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
					exceedPercent := (float64(numReq)/float64(newReport.Rules.ReqPerTimeThreshold) - 1) * 100
					for _, recipient := range entry.conf.Recipients {
						link := fmt.Sprintf(
							"%s/alarm/%s/confirmation?reviewer=%s",
							aticker.alarmConf.ConfirmationBaseURL, newReport.ReviewCode, recipient,
						)
						mail.SendNotification(
							client,
							aticker.alarmConf.Sender,
							[]string{recipient},
							fmt.Sprintf(
								"CNC APIGuard - překročení přístupů k API o %01.2f%% u služby '%s'",
								exceedPercent, entry.service,
							),
							fmt.Sprintf(
								"Byl detekován velký počet API dotazů na službu '%s' od uživatele ID %d: %d za posledních %d sekund"+
									"Max. povolený limit pro tuto službu je %d dotazů za %d sekund.",
								entry.service, userID, numReq, newReport.Rules.ReqCheckingIntervalSecs,
								newReport.Rules.ReqPerTimeThreshold,
								newReport.Rules.ReqCheckingIntervalSecs,
							),
							fmt.Sprintf(
								"Detaily získáte a hlášení potvrdíte kliknutím na odkaz: <a href=\"%s\">%s</a>",
								link, link,
							),
						)
					}

				}()
				entry.clientRequests[userID] = 0
				log.Warn().Msgf("detected high activity for service %s and user %d", entry.service, userID)
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

func (aticker *AlarmTicker) Register(service string, conf Conf) chan<- RequestInfo {
	aticker.servicesLock.Lock()
	aticker.clients[service] = &serviceEntry{
		service:        service,
		conf:           conf,
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
	reviewerMail := req.URL.Query().Get("reviewer")
	for _, report := range aticker.reports {
		if report.ReviewCode == alarmID {
			err := report.ConfirmReviewViaEmail(alarmID, reviewerMail)
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
			}
			uniresp.WriteJSONResponse(w, map[string]any{
				"confirmed": true,
				"report":    report,
			})
			return
		}
	}
	uniresp.WriteJSONErrorResponse(
		w,
		uniresp.NewActionError("confirmation key not found"),
		http.StatusNotFound,
	)
}

func NewAlarmTicker(loc *time.Location, alarmConf MailConf, usersTableName string) *AlarmTicker {
	return &AlarmTicker{
		clients:        make(map[string]*serviceEntry),
		counter:        make(chan RequestInfo, 1000),
		location:       loc,
		alarmConf:      alarmConf,
		usersTableName: usersTableName,
	}
}
