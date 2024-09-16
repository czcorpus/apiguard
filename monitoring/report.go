// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"apiguard/guard"
	"apiguard/users"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConfirmationKeyNotFound       = errors.New("confirmation key not foud")
	ErrMissingReviewerIdentification = errors.New("missing reviewer identification")
)

type reportFloat float64

func (rf reportFloat) String() string {
	return fmt.Sprintf("%01.2f", rf)
}

type Reviewer struct {
	UserID   int       `json:"userId"`
	Email    string    `json:"email"`
	Reviewed time.Time `json:"datetime,omitempty"`
}

type AlarmReport struct {
	RequestInfo guard.RequestInfo `json:"requestInfo"`
	Alarm       AlarmConf         `json:"-"`
	Rules       Limit             `json:"rules"`
	Created     time.Time         `json:"created"`
	Reviewed    time.Time         `json:"reviewed"`
	ReviewCode  string            `json:"reviewCode"`
	UserInfo    *users.User       `json:"userInfo"`
	Reviews     []Reviewer        `json:"reviews"`
	location    *time.Location
}

func (report *AlarmReport) AttachUserInfo(table *users.UsersTable) error {
	userInfo, err := table.UserInfo(report.RequestInfo.UserID)
	if err == nil && userInfo != nil {
		report.UserInfo = userInfo
	}
	return err
}

func (report *AlarmReport) MarshalJSON() ([]byte, error) {
	reviewers := make([]string, len(report.Reviews))
	for i, rev := range report.Reviews {
		reviewers[i] = rev.Email
		// TODO handle "no email but ID" cases
	}
	var reviewed2 *time.Time
	if !report.Reviewed.IsZero() {
		reviewed2 = &report.Reviewed
	}
	return json.Marshal(
		struct {
			RequestInfo guard.RequestInfo `json:"requestInfo"`
			Rules       AlarmConf         `json:"rules"`
			Created     time.Time         `json:"created"`
			Reviewed    *time.Time        `json:"reviewed"`
			ReviewCode  string            `json:"reviewCode"`
			UserInfo    *users.User       `json:"userInfo,omitempty"`
			Reviewers   []string          `json:"reviewers"`
		}{
			RequestInfo: report.RequestInfo,
			Rules:       report.Alarm,
			Created:     report.Created,
			Reviewed:    reviewed2,
			ReviewCode:  report.ReviewCode,
			UserInfo:    report.UserInfo,
			Reviewers:   reviewers,
		},
	)

}

func (report *AlarmReport) IsReviewed() bool {
	return len(report.Reviews) > 0
}

func (report *AlarmReport) ConfirmReviewViaEmail(alarmID string, reviewerMail string) error {
	if alarmID != report.ReviewCode {
		return ErrConfirmationKeyNotFound
	}
	if reviewerMail == "" {
		return ErrMissingReviewerIdentification
	}
	report.Reviews = append(
		report.Reviews,
		Reviewer{
			Email:    reviewerMail,
			Reviewed: time.Now().In(report.location),
		},
	)
	if len(report.Reviews) == 1 {
		report.Reviewed = time.Now().In(report.location)
	}
	return nil
}

func (report *AlarmReport) ConfirmReviewViaID(alarmID string, reviewerID int) error {
	if alarmID != report.ReviewCode {
		return ErrConfirmationKeyNotFound
	}
	if reviewerID <= 0 {
		return ErrMissingReviewerIdentification
	}
	report.Reviews = append(
		report.Reviews,
		Reviewer{
			UserID:   reviewerID,
			Reviewed: time.Now().In(report.location),
		},
	)
	if len(report.Reviews) == 1 {
		report.Reviewed = time.Now().In(report.location)
	}
	return nil
}

func (report *AlarmReport) ExceedPercent() reportFloat {
	return (reportFloat(report.RequestInfo.NumRequests)/reportFloat(report.Rules.ReqPerTimeThreshold) - 1) * 100
}

func generateReviewCode() string {
	id := uuid.New()
	sum := sha1.Sum([]byte(id.String()))
	return hex.EncodeToString(sum[:])
}

func NewAlarmReport(reqInfo guard.RequestInfo, alarmConf AlarmConf, rules Limit, loc *time.Location) *AlarmReport {
	return &AlarmReport{
		Reviews:     make([]Reviewer, 0, 5),
		Created:     time.Now().In(loc),
		RequestInfo: reqInfo,
		ReviewCode:  generateReviewCode(),
		Alarm:       alarmConf,
		Rules:       rules,
		location:    loc,
	}
}
