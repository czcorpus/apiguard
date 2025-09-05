// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package frodo

import (
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/services/cnc"
	"fmt"
)

type FrodoProxy struct {
	*cnc.Proxy
}

func NewFrodoProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*FrodoProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create Frodo proxy: %w", err)
	}
	return &FrodoProxy{
		Proxy: proxy,
	}, nil
}
