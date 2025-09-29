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

package ujc

import (
	"encoding/json"
	"fmt"

	"github.com/czcorpus/apiguard-common/globctx"
	"github.com/czcorpus/apiguard-ext/services/ujc/assc"
	"github.com/czcorpus/apiguard-ext/services/ujc/cja"
	"github.com/czcorpus/apiguard-ext/services/ujc/kla"
	"github.com/czcorpus/apiguard-ext/services/ujc/lguide"
	"github.com/czcorpus/apiguard-ext/services/ujc/neomat"
	"github.com/czcorpus/apiguard-ext/services/ujc/psjc"
	"github.com/czcorpus/apiguard-ext/services/ujc/ssjc"
	"github.com/czcorpus/apiguard/config"
	"github.com/czcorpus/apiguard/guard/tlmtr"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func InitUJCService(
	ctx *globctx.Context,
	sid int,
	servConf config.GeneralServiceConf,
	globalConf *config.Configuration,
	apiRoutes gin.IRoutes,
) error {

	switch servConf.Type {

	// "Jazyková příručka ÚJČ"
	case "languageGuide":
		var typedConf lguide.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
		}
		if err := typedConf.Validate("languageGuide"); err != nil {
			return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (languageGuide): %w", sid, err)
		}
		langGuideActions := lguide.NewLanguageGuideActions(
			ctx,
			fmt.Sprintf("%d/language-guide", sid),
			&typedConf,
			&globalConf.Botwatch,
			globalConf.Telemetry,
			globalConf.ServerReadTimeoutSecs,
			guard,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/language-guide", sid),
			langGuideActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for LanguageGuide enabled")

	// "Akademický slovník současné češtiny"
	case "assc":
		var typedConf assc.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
		}
		if err := typedConf.Validate("assc"); err != nil {
			return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (assc): %w", sid, err)
		}
		asscActions := assc.NewASSCActions(
			ctx,
			fmt.Sprintf("%d/assc", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/assc", sid),
			asscActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for ASSC enabled")

	// "Slovník spisovného jazyka českého"
	case "ssjc":
		var typedConf ssjc.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
		}
		if err := typedConf.Validate("ssjc"); err != nil {
			return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (ssjc): %w", sid, err)
		}
		ssjcActions := ssjc.NewSSJCActions(
			ctx,
			fmt.Sprintf("%d/ssjc", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/ssjc", sid),
			ssjcActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for SSJC enabled")

	// "Příruční slovník jazyka českého"
	case "psjc":
		var typedConf psjc.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
		}
		if err := typedConf.Validate("psjc"); err != nil {
			return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (psjc): %w", sid, err)
		}
		psjcActions := psjc.NewPSJCActions(
			ctx,
			fmt.Sprintf("%d/psjc", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/psjc", sid),
			psjcActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for PSJC enabled")

	// "Kartotéka lexikálního archivu"
	case "kla":
		var typedConf kla.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
		}
		if err := typedConf.Validate("kla"); err != nil {
			return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (kla): %w", sid, err)
		}
		klaActions := kla.NewKLAActions(
			ctx,
			fmt.Sprintf("%d/kla", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/kla", sid),
			klaActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for KLA enabled")

	// "Neomat"
	case "neomat":
		var typedConf neomat.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
		}
		if err := typedConf.Validate("neomat"); err != nil {
			return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (neomat): %w", sid, err)
		}
		neomatActions := neomat.NewNeomatActions(
			ctx,
			fmt.Sprintf("%d/neomat", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/neomat", sid),
			neomatActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for Neomat enabled")

	// "Český jazykový atlas"
	case "cja":
		var typedConf cja.Conf
		if err := json.Unmarshal(servConf.Conf, &typedConf); err != nil {
			return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
		}
		if err := typedConf.Validate("cja"); err != nil {
			return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
		}
		guard, err := tlmtr.New(ctx, &globalConf.Botwatch, globalConf.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize service %d (cja): %w", sid, err)
		}
		cjaActions := cja.NewCJAActions(
			ctx,
			fmt.Sprintf("%d/cja", sid),
			&typedConf,
			guard,
			globalConf.ServerReadTimeoutSecs,
		)
		apiRoutes.GET(
			fmt.Sprintf("/service/%d/cja", sid),
			cjaActions.Query,
		)
		log.Info().Int("sid", sid).Msg("Proxy for CJA enabled")
	}
	return nil
}