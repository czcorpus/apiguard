// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/guard/sessionmap"
	"apiguard/services/cnc"
	"fmt"
)

type MQueryProxy struct {
	*cnc.CoreProxy
}

func NewMQueryProxy(
	globalCtx *ctx.GlobalContext,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard *sessionmap.Guard,
	reqCounter chan<- guard.RequestInfo,
) (*MQueryProxy, error) {
	proxy, err := cnc.NewCoreProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQuery proxy: %w", err)
	}
	return &MQueryProxy{
		CoreProxy: proxy,
	}, nil
}
