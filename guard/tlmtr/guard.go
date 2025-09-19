// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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

//go:build closed

package tlmtr

import (
	"github.com/czcorpus/apiguard-common/botwatch"
	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-common/guard"
	"github.com/czcorpus/apiguard-common/telemetry"

	"github.com/czcorpus/apiguard-ext/aeguard"
)

func New(
	ctx *globctx.Context,
	conf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
) (guard.ServiceGuard, error) {
	return aeguard.New(ctx, conf, telemetryConf)
}
