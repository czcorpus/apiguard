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

package globctx

import (
	"context"
	"database/sql"
	"time"

	"github.com/czcorpus/apiguard/cache"
	"github.com/czcorpus/apiguard/common"
	"github.com/czcorpus/apiguard/reporting"
	"github.com/czcorpus/apiguard/telemetry"
)

type BackendLoggers map[string]*BackendLogger

func (bl BackendLoggers) Get(serviceKey string) *BackendLogger {
	lg, ok := bl[serviceKey]
	if ok {
		return lg
	}
	return bl["default"]
}

// Context provides access to shared resources and information needed by different
// part of the application. It is OK to pass it by value as the properties of the struct
// are pointers themselves (if needed).
// It also fulfills context.Context interface so it can be used along with some existing
// context.
type Context struct {
	TimezoneLocation *time.Location
	BackendLoggers   BackendLoggers
	CNCDB            *sql.DB
	TelemetryDB      telemetry.Storage
	ReportingWriter  reporting.ReportingWriter
	Cache            cache.Cache
	wCtx             context.Context
	AnonymousUserIDs common.AnonymousUsers
}

func (gc *Context) Deadline() (deadline time.Time, ok bool) {
	return gc.wCtx.Deadline()
}

func (gc *Context) Done() <-chan struct{} {
	return gc.wCtx.Done()
}

func (gc *Context) Err() error {
	return gc.wCtx.Err()
}

func (gc *Context) Value(key any) any {
	return gc.wCtx.Value(key)
}

func NewGlobalContext(ctx context.Context) *Context {
	return &Context{wCtx: ctx}
}
