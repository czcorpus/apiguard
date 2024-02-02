// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"apiguard/common"
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/guard/dflt"
	"apiguard/guard/sessionmap"
	"apiguard/services/cnc"
)

func NewGuard(
	name cnc.GuardType,
	delayStats *guard.DelayStats,
	globalCtx *ctx.GlobalContext,
	internalSessCookieName string,
	externalSessCookieName string,
	anonymousUserID common.UserID,
) guard.ReqAnalyzer {
	switch name {
	case cnc.GuardTypeDefault:
		return dflt.New(globalCtx.CNCDB, delayStats, externalSessCookieName)
	case cnc.GuardTypeSessionMap:
		return sessionmap.New(
			globalCtx, delayStats, internalSessCookieName, externalSessCookieName, anonymousUserID)
	case cnc.GuardTypeNull:
		return nil // TODO implement NullGuard
	}
	return nil
}
