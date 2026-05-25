// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	apiRepConfDefaultRefreshInterval = "1h"
)

type APIReportingConf struct {
	HeaderName string `json:"headerName"`
	Secret     string `json:"secret"`

	// ExpectedValue is an alternative to the Secret for web applications
	// which are unable to provide hmac secure "tags". Both values are
	// mutually exclusive and APIGuard will check whether both values
	// are not set at the same time.
	ExpectedValue   string `json:"expectedValue"`
	AppID           string `json:"appId"`
	RefreshInterval string `json:"refreshInterval"`
}

func (repConf APIReportingConf) ValidateAndDefaults() error {
	if repConf.Secret != "" && repConf.ExpectedValue != "" {
		return errors.New("APIReportingConf error - cannot use both 'secret' and 'expectedValue' at the same time")
	}
	if repConf.RefreshInterval == "" {
		repConf.RefreshInterval = apiRepConfDefaultRefreshInterval
		log.Warn().Str("default", apiRepConfDefaultRefreshInterval).Msg("'refreshInterval' not set, using default")
	}
	if (repConf.Secret != "" || repConf.ExpectedValue != "") && repConf.HeaderName == "" {
		return errors.New("APIReportingConf error - missing 'headerName'")
	}
	return nil
}

// ---------------------------

type Limit struct {
	ReqPerTimeThreshold     int `json:"reqPerTimeThreshold"`
	ReqCheckingIntervalSecs int `json:"reqCheckingIntervalSecs"`
	BurstLimit              int `json:"burstLimit"`
}

func (m Limit) ReqCheckingInterval() time.Duration {
	return time.Duration(m.ReqCheckingIntervalSecs) * time.Second
}

func (m Limit) NormLimitPerSec() rate.Limit {
	return rate.Limit(float64(m.ReqPerTimeThreshold) / float64(m.ReqCheckingIntervalSecs))
}

// --------------------------

type GeneralProxyConf struct {
	BackendURL          string
	FrontendURL         string
	ReqTimeoutSecs      int
	IdleConnTimeoutSecs int
	Limits              []Limit
	APIReporting        APIReportingConf
}

// ---------------------------

type GlobalContext struct {
	TimezoneLocation *time.Location
}

// -------------------------------

func WriteError(ctx *gin.Context, err error, status int) {
	if ctx.Request.Header.Get("content-type") == "application/json" ||
		ctx.Request.Header.Get("content-type") == "text/event-stream" {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to proxy request: %s", err),
			status,
		)

	} else {
		http.Error(
			ctx.Writer,
			fmt.Sprintf("Failed to proxy request: %s", err),
			status,
		)
	}
}

// --------------------------------

type MultiStatusCode []int

// Result provides a single HTTP status code
// based on particular status codes of some sub-requests
func (msc MultiStatusCode) Result() int {
	var maxCode int
	var has200 bool
	for _, v := range msc {
		if v == http.StatusOK {
			has200 = true
		}
		if v > maxCode {
			maxCode = v
		}
	}
	if has200 && maxCode < 500 {
		return http.StatusOK
	}
	if maxCode >= 500 {
		return http.StatusBadGateway
	}
	return maxCode
}

// -------------------------------------
