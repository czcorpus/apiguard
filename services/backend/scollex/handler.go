// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package scollex

import (
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/services/cnc"
	"fmt"
)

type ScollexProxy struct {
	*cnc.Proxy
}

func NewScollexProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*ScollexProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create Scollex proxy: %w", err)
	}
	return &ScollexProxy{
		Proxy: proxy,
	}, nil
}
