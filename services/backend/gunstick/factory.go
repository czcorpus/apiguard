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

package gunstick

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/czcorpus/apiguard/guard/dflt"
	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/proxy/public"
	"github.com/czcorpus/apiguard/services"
	"github.com/czcorpus/apiguard/srvfactory"
	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/rs/zerolog/log"
)

func init() {
	srvfactory.RegisterServiceInitializer("gunstick", create)
}

func create(args services.InitArgs) error {
	var typedConf Conf
	if err := json.Unmarshal(args.RawConf, &typedConf); err != nil {
		return fmt.Errorf("failed to initialize service %d (gunstick): %w", args.SID, err)
	}
	if err := typedConf.Validate("gunstick"); err != nil {
		return fmt.Errorf("failed to initialize service %d (gunstick): %w", args.SID, err)
	}
	client := httpclient.New(
		httpclient.WithFollowRedirects(),
		httpclient.WithInsecureSkipVerify(),
		httpclient.WithIdleConnTimeout(time.Duration(60)*time.Second),
	)
	grd := dflt.New(
		args.Ctx,
		args.GlobalConf.CNCAuth.SessionCookieName,
		typedConf.SessionValType,
		typedConf.Limits,
	)
	go grd.Run()
	backendURL, err := url.Parse(typedConf.BackendURL)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (gunstick): %w", args.SID, err)
	}
	frontendURL, err := url.Parse(typedConf.FrontendURL)
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (gunstick): %w", args.SID, err)
	}
	coreProxy, err := proxy.NewCoreProxy(typedConf.GetCoreConf())
	if err != nil {
		return fmt.Errorf("failed to initialize service %d (gunstick): %w", args.SID, err)
	}
	gunstickActions := public.NewProxy(
		args.Ctx,
		coreProxy,
		args.SID,
		client,
		grd.ExposeAsCounter(),
		grd,
		public.PublicAPIProxyOpts{
			ServicePath:     fmt.Sprintf("/service/%d/gunstick", args.SID),
			ServiceKey:      fmt.Sprintf("%d/gunstick", args.SID),
			BackendURL:      backendURL,
			FrontendURL:     frontendURL,
			ReadTimeoutSecs: args.GlobalConf.ServerReadTimeoutSecs,
		},
	)
	args.APIRoutes.Any(
		fmt.Sprintf("/service/%d/gunstick/*path", args.SID),
		gunstickActions.AnyPath,
	)
	log.Info().Int("args.SID", args.SID).Msg("Proxy for Gunstick enabled")
	return nil
}
