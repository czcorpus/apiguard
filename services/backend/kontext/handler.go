// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/services/cnc"
	"fmt"
)

type KonTextProxy struct {
	*cnc.CoreProxy
}

func NewKontextProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guard.RequestInfo,
) (*KonTextProxy, error) {
	proxy, err := cnc.NewCoreProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create KonText proxy: %w", err)
	}
	return &KonTextProxy{
		CoreProxy: proxy,
	}, nil
}
