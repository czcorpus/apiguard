// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// WriteJSONResponse writes 'value' to an HTTP response encoded as JSON
func WriteJSONResponse(w http.ResponseWriter, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(jsonAns)
}

// ActionError represents a basic user action error (e.g. a wrong parameter,
// non-existing record etc.)
type ActionError struct {
	error
}

// MarshalJSON serializes the error to JSON
func (me ActionError) MarshalJSON() ([]byte, error) {
	return json.Marshal(me.Error())
}

// NewActionErrorFrom is the default factory for creating an ActionError instance
// out of an existing error
func NewActionErrorFrom(origErr error) ActionError {
	return ActionError{origErr}
}

// NewActionError creates an Action error from provided message using
// a newly defined general error as an original error
func NewActionError(msg string, args ...interface{}) ActionError {
	return ActionError{fmt.Errorf(msg, args...)}
}

// ErrorResponse describes a wrapping object for all error HTTP responses
type ErrorResponse struct {
	Error   *ActionError `json:"error"`
	Details []string     `json:"details"`
}

// WriteJSONErrorResponse writes 'aerr' to an HTTP error response  as JSON
func WriteJSONErrorResponse(w http.ResponseWriter, aerr ActionError, status int, details ...string) {
	ans := &ErrorResponse{Error: &aerr, Details: details}
	jsonAns, err := json.Marshal(ans)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(status)
	w.Write(jsonAns)
}

type ReqAnalyzer interface {
	CalcDelay(req *http.Request) (time.Duration, error)
	RegisterDelayLog(respDelay time.Duration) error
	UserInducedResponseStatus(req *http.Request) (int, int, error)
}

func RestrictResponseTime(w http.ResponseWriter, req *http.Request, readTimeoutSecs int, analyzer ReqAnalyzer) error {
	respDelay, err := analyzer.CalcDelay(req)
	if err != nil {
		WriteJSONErrorResponse(
			w,
			NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		log.Error().Err(err).Msg("failed to analyze client")
		return err
	}
	log.Debug().Msgf("Client is going to wait for %v", respDelay)
	if respDelay.Seconds() >= float64(readTimeoutSecs) {
		WriteJSONErrorResponse(
			w,
			NewActionError("service overloaded"),
			http.StatusServiceUnavailable,
		)
		return err
	}
	go analyzer.RegisterDelayLog(respDelay)
	time.Sleep(respDelay)
	return nil
}

func GetSessionKey(req *http.Request, cookieName string) string {
	var cookieValue string
	for _, cookie := range req.Cookies() {
		if cookie.Name == cookieName {
			cookieValue = cookie.Value
			break
		}
	}
	return cookieValue
}
