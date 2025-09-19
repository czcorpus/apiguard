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

package mquery

import (
	"fmt"

	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-common/guard"
	guardImpl "github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/services/cnc"
)

type MQueryProxy struct {
	*cnc.Proxy
}

func NewMQueryProxy(
	globalCtx *globctx.Context,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	guard guard.ServiceGuard,
	reqCounter chan<- guardImpl.RequestInfo,
) (*MQueryProxy, error) {
	proxy, err := cnc.NewProxy(globalCtx, conf, gConf, guard, reqCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQuery proxy: %w", err)
	}
	return &MQueryProxy{
		Proxy: proxy,
	}, nil
}
