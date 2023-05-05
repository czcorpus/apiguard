// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"apiguard/cncdb"
	"apiguard/common"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/mail"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const (
	clientsDataFile                = "clients.gob"
	reportsDataFile                = "reports.gob"
	dfltRecCountCleanupProbability = 0.5
)

type reqCounterItem struct {
	created time.Time
}

type userCounters map[common.UserID]*userLimitInfo

type serviceEntry struct {
	Conf           AlarmConf
	limits         map[common.CheckInterval]int
	Service        string
	ClientRequests userCounters
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
	counts := service.ClientRequests[userID]
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
			err := newReport.AttachUserInfo(cncdb.NewUsersTable(
				aticker.db, aticker.userTableProps))
			if err != nil {
				newReport.UserInfo = &cncdb.User{
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
	for _, data := range service.ClientRequests {
		oldNumReq += len(data.Requests)
		keep := make([]reqCounterItem, 0, 1000)
		for _, req := range data.Requests {
			if req.created.After(oldestDate) {
				keep = append(keep, req)
			}
		}
		data.Requests = keep
		newNumReq += len(keep)
	}
	cleanupSummary := "Data size unchanged"
	if newNumReq < oldNumReq {
		cleanupSummary = fmt.Sprintf("Reduced number of records from %d to %d", oldNumReq, newNumReq)
	}
	log.Info().
		Int("numOldRec", oldNumReq).
		Int("numNewRec", newNumReq).
		Msgf("Performed old requests cleanup. %s", cleanupSummary)
}

func (aticker *AlarmTicker) Run(quitChan <-chan os.Signal) {
	for item := range aticker.counter {
		if entry, ok := aticker.clients[item.Service]; ok {
			_, ok := entry.ClientRequests[item.UserID]
			if !ok {
				entry.ClientRequests[item.UserID] = &userLimitInfo{
					Requests: make([]reqCounterItem, 0, 1000),
					Reported: make(map[common.CheckInterval]bool),
				}
			}
			entry.ClientRequests[item.UserID].Requests = append(
				entry.ClientRequests[item.UserID].Requests,
				reqCounterItem{
					created: time.Now().In(aticker.location),
				},
			)
		}
		go func(item RequestInfo) {
			aticker.checkServiceUsage(
				aticker.clients[item.Service],
				item.UserID,
			)
		}(item)
		// from time to time, remove older records
		if rand.Float64() < aticker.clients[item.Service].Conf.RecCounterCleanupProbability {
			go aticker.clearOldRecords(aticker.clients[item.Service])
		}
	}
}

func (aticker *AlarmTicker) Register(service string, conf AlarmConf, limits []Limit) chan<- RequestInfo {
	aticker.servicesLock.Lock()
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
		ClientRequests: make(userCounters),
	}
	for _, limit := range limits {
		sEntry.limits[common.CheckInterval(limit.ReqCheckingInterval())] = limit.ReqPerTimeThreshold
		sEntry.ClientRequests = make(userCounters)
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
