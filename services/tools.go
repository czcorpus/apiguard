package services

import (
	"encoding/json"
	"fmt"
	"net/http"
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
