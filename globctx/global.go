// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package globctx

import (
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/telemetry"
	"context"
	"database/sql"
	"time"
)

// Context provides access to shared resources and information needed by different
// part of the application. It is OK to pass it by value as the properties of the struct
// are pointers themselves (if needed).
// It also fulfills context.Context interface so it can be used along with some existing
// context.
type Context struct {
	TimezoneLocation *time.Location
	BackendLogger    *BackendLogger
	CNCDB            *sql.DB
	TelemetryDB      telemetry.Storage
	ReportingWriter  reporting.ReportingWriter
	Cache            proxy.Cache
	wCtx             context.Context
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
