// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/guard/userdb"
	"apiguard/services/cnc"
)

type MQueryProxy struct {
	cnc.CoreProxy
}

func NewMQueryProxy(
	globalCtx *ctx.GlobalContext,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	analyzer *userdb.CNCUserAnalyzer,
	reqCounter chan<- guard.RequestInfo,
) *MQueryProxy {
	return &MQueryProxy{
		CoreProxy: *cnc.NewCoreProxy(globalCtx, conf, gConf, analyzer, reqCounter),
	}
}
