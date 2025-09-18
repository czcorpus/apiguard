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

package kontext

import (
	"apiguard/config"
	"apiguard/services/cnc"
	"fmt"
	"net/url"

	"github.com/rs/zerolog/log"
)

type Conf struct {
	cnc.ProxyConf
	UseSimplifiedConcReq bool `json:"useSimplifiedConcReq"`
}

func (conf *Conf) Validate(name string) error {
	if err := conf.ProxyConf.Validate(name); err != nil {
		return err
	}
	if conf.ReqTimeoutSecs == 0 {
		conf.ReqTimeoutSecs = config.DfltProxyReqTimeoutSecs
		log.Warn().Msgf(
			"%s: missing reqTimeoutSecs, setting %d", name, config.DfltProxyReqTimeoutSecs)

	} else if conf.ReqTimeoutSecs < 0 {
		return fmt.Errorf("%s: invalid reqTimeoutSecs value: %d", name, conf.ReqTimeoutSecs)
	}
	if conf.IdleConnTimeoutSecs == 0 {
		conf.IdleConnTimeoutSecs = config.DfltIdleConnTimeoutSecs
		log.Warn().Msgf(
			"%s: missing idleConnTimeoutSecs, setting %d", name, config.DfltIdleConnTimeoutSecs)

	} else if conf.IdleConnTimeoutSecs < 0 {
		return fmt.Errorf("%s: invalid idleConnTimeoutSecs value: %d", name, conf.IdleConnTimeoutSecs)
	}
	if conf.BackendURL == "" {
		return fmt.Errorf("%s: missing backendUrl", name)
	}
	if conf.FrontendURL == "" {
		return fmt.Errorf("%s: missing frontendUrl", name)
	}
	return nil
}

type querySubmitArgs struct {
	Queries      []any    `json:"queries"`
	Maincorp     string   `json:"maincorp"`
	Usesubcorp   string   `json:"usesubcorp"`
	Viewmode     string   `json:"viewmode"`
	Pagesize     int      `json:"pagesize"`
	Shuffle      int      `json:"shuffle"`
	Attrs        []string `json:"attrs"`
	CtxAttrs     []string `json:"ctxattrs"`
	AttrVmode    string   `json:"attr_vmode"`
	BaseViewattr string   `json:"base_viewattr"`
	Structs      []string `json:"structs"`
	Refs         []string `json:"refs"`
	Fromp        int      `json:"fromp"`
	TextTypes    any      `json:"textTypes"`
	Context      any      `json:"context"`
	KWICLeftCtx  int      `json:"kwicleftctx"`
	KWICRightCtx int      `json:"kwicrightctx"`
	Type         string   `json:"concQueryArgs"`
}

type viewActionArgs struct {
	Corpname     string `json:"corpname"`
	Usesubcorp   string `json:"usesubcorp"`
	Maincorp     string `json:"maincorp"`
	Q            string `json:"q"`
	KWICLeftCtx  string `json:"kwicleftctx"`
	KWICRightCtx string `json:"kwicrightctx"`
	Pagesize     string `json:"pagesize"`
	Fromp        string `json:"fromp"`
	AttrVmode    string `json:"attr_vmode"`
	Attr         string `json:"attrs"`
	Viewmode     string `json:"viewmode"`
	Shuffle      string `json:"shuffle"`
	Refs         string `json:"refs"`
	Format       string `json:"format"`
}

type querySubmitResponse struct {
	Q                   []string `json:"Q"`
	ConcPersistenceOpID string   `json:"conc_persistence_op_id"`
	NumLinesInGroups    int      `json:"num_lines_in_groups"`
	LinesGroupsNumbers  []int    `json:"lines_groups_numbers"`
	QueryOverview       []any    `json:"query_overview"`
	Finished            bool     `json:"finished"`
	Size                int      `json:"size"`
	Messages            []any    `json:"messages"`
}

func (va *viewActionArgs) ToURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("corpname", va.Corpname)
	q.Add("usesubcorp", va.Usesubcorp)
	q.Add("maincorp", va.Maincorp)
	q.Add("q", va.Q)
	q.Add("kwicleftctx", va.KWICLeftCtx)
	q.Add("kwicrightctx", va.KWICRightCtx)
	q.Add("pagesize", va.Pagesize)
	q.Add("fromp", va.Fromp)
	q.Add("attr_vmode", va.AttrVmode)
	q.Add("attrs", va.Attr)
	q.Add("viewmode", va.Viewmode)
	q.Add("shuffle", va.Shuffle)
	q.Add("refs", va.Refs)
	q.Add("format", va.Format)
	return q.Encode()
}
