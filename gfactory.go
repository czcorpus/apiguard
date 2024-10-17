// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/common"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/dflt"
	"apiguard/guard/null"
	"apiguard/guard/sessionmap"
	"apiguard/proxy"
	"apiguard/services/cnc"
	"time"
)

func NewGuard(
	name cnc.GuardType,
	globalCtx *globctx.Context,
	internalSessCookieName string,
	externalSessCookieName string,
	confLimits []proxy.Limit,
	anonymousUserIDs common.AnonymousUsers,
	loc *time.Location,
) guard.ServiceGuard {
	switch name {
	case cnc.GuardTypeDefault:
		return dflt.New(
			globalCtx,
			externalSessCookieName,
		)
	case cnc.GuardTypeSessionMap:
		return sessionmap.New(
			globalCtx,
			internalSessCookieName,
			externalSessCookieName,
			confLimits,
		)
	case cnc.GuardTypeNull:
		return null.New()
	}
	return nil
}
