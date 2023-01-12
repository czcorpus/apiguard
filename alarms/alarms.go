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
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/czcorpus/uniresp"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const (
	clientsDataFile = "clients.gob"
	reportsDataFile = "reports.gob"
)

type perLimitCounter map[int]int // userID -> num requests

type serviceEntry struct {
	Conf           AlarmConf
	limits         map[int]int // ReqCheckingIntervalSecs -> max req. limit
	Service        string
	ClientRequests map[int]perLimitCounter // ReqCheckingIntervalSecs -> user requests counts
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

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

type AlarmTicker struct {
	db             *sql.DB
	alarmConf      MailConf
	clients        map[string]*serviceEntry //save
	servicesLock   sync.Mutex
	ticker         *time.Ticker
	counter        chan RequestInfo
	reports        []*AlarmReport //save
	location       *time.Location
	userTableProps cncdb.UserTableProps
	statusDataDir  string
}

func (aticker *AlarmTicker) SaveAttributes() error {
	file, err := os.Create(path.Join(aticker.statusDataDir, clientsDataFile))
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(aticker.clients)
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}

	file, err = os.Create(path.Join(aticker.statusDataDir, reportsDataFile))
	if err != nil {
		return err
	}
	encoder = gob.NewEncoder(file)
	err = encoder.Encode(aticker.reports)
	if err != nil {
		return err
	}
	err = file.Close()
	log.Debug().Msg("Alarm attributes saved")
	return err
}

func (aticker *AlarmTicker) LoadAttributes() error {
	file_path := path.Join(aticker.statusDataDir, clientsDataFile)
	if fileExists(file_path) {
		file, err := os.Open(file_path)
		if err != nil {
			return err
		}
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(&aticker.clients)
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	}

	file_path = path.Join(aticker.statusDataDir, reportsDataFile)
	if fileExists(file_path) {
		file, err := os.Open(file_path)
		if err != nil {
			return err
		}
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(&aticker.reports)
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	}
	log.Debug().Msg("Alarm attributes loaded")
	return nil
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
	for checkIntervalSecs, plCounter := range entry.ClientRequests {
		if unixTime%int64(checkIntervalSecs) == 0 {
			for userID, numReq := range plCounter {
				if plCounter[checkIntervalSecs] > entry.limits[checkIntervalSecs] {
					newReport := NewAlarmReport(
						RequestInfo{
							Service:     entry.Service,
							NumRequests: numReq,
							UserID:      userID,
						},
						entry.Conf,
						Limit{
							ReqCheckingIntervalSecs: checkIntervalSecs,
							ReqPerTimeThreshold:     entry.limits[checkIntervalSecs],
						},
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

						for _, recipient := range entry.Conf.Recipients {
							log.Debug().Msgf("about to send a notification e-mail to %s", recipient)
							page := aticker.createConfirmationPageURL(newReport, recipient)
							err := mail.SendNotification(
								client,
								aticker.location,
								aticker.alarmConf.Sender,
								[]string{recipient},
								fmt.Sprintf(
									"CNC APIGuard - překročení přístupů k API o %01.1f%% u služby '%s'",
									newReport.ExceedPercent(), entry.Service,
								),
								fmt.Sprintf(
									"Byl detekován velký počet API dotazů na službu '%s' od uživatele ID %d: %d za posledních %d sekund.<br /> "+
										"Max. povolený limit pro tuto službu je %d dotazů za %d sekund.",
									entry.Service, userID, numReq, newReport.Rules.ReqCheckingIntervalSecs,
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
					entry.ClientRequests[checkIntervalSecs][userID] = 0
					log.Warn().Msgf("detected high activity for service %s and user %d", entry.Service, userID)
				}
			}
		}
	}
}

func (aticker *AlarmTicker) Run(quitChan <-chan os.Signal) {
	go func() {
		for item := range aticker.counter {
			if entry, ok := aticker.clients[item.Service]; ok {
				for _, counter := range entry.ClientRequests {
					counter[item.UserID] += item.NumRequests
				}
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
	sEntry := &serviceEntry{
		Service:        service,
		Conf:           conf,
		limits:         make(map[int]int),
		ClientRequests: make(map[int]perLimitCounter),
	}
	for _, limit := range limits {
		sEntry.limits[limit.ReqCheckingIntervalSecs] = limit.ReqPerTimeThreshold
		sEntry.ClientRequests[limit.ReqCheckingIntervalSecs] = make(map[int]int)
	}
	aticker.clients[service] = sEntry
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
	statusDataDir string,
) *AlarmTicker {
	return &AlarmTicker{
		db:             db,
		clients:        make(map[string]*serviceEntry),
		counter:        make(chan RequestInfo, 1000),
		location:       loc,
		alarmConf:      alarmConf,
		userTableProps: userTableProps,
		statusDataDir:  statusDataDir,
	}
}
