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

package services

import (
	"encoding/json"
	"net/http"

	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/monitoring"
	"github.com/gin-gonic/gin"
)

// VersionInfo provides a detailed information about the actual build
type VersionInfo struct {
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
	GitCommit string `json:"gitCommit"`
}

type InitArgs struct {
	Ctx        *globctx.Context
	SID        int
	RawConf    json.RawMessage
	GlobalConf *config.Configuration
	APIRoutes  *gin.RouterGroup
	Engine     http.Handler
	Alarm      *monitoring.AlarmTicker
}
