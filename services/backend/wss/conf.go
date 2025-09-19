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

package wss

import (
	"fmt"

	"github.com/czcorpus/apiguard/guard"
	"github.com/czcorpus/apiguard/guard/token"
	"github.com/czcorpus/apiguard/services/cnc"
)

type Conf struct {
	cnc.ProxyConf
	GuardType       guard.GuardType   `json:"guardType"`
	TokenHeaderName string            `json:"tokenHeaderName"`
	Tokens          []token.TokenConf `json:"tokens"`
}

func (c *Conf) Validate(context string) error {
	if err := c.ProxyConf.Validate(context); err != nil {
		return err
	}
	if c.GuardType == guard.GuardTypeToken && len(c.Tokens) == 0 {
		return fmt.Errorf("no tokens defined for token wss - the service won't be accessible")
	}
	return nil
}

// ---------

type ttCollsFreqsArgs struct {
	TileID    int      `json:"tileId"`
	TextTypes []string `json:"textTypes"`
	Dataset   string   `json:"dataset"`
	Word      string   `json:"word"`
	PoS       string   `json:"pos"`
	Limit     int      `json:"limit"`
}

// --------

type streamResponse struct {
	Parts map[string]collResponse `json:"parts"`
	Error string                  `json:"error,omitempty"`
}

// -----

type lemmaInfo struct {
	Value string `json:"value"`
	PoS   string `json:"pos"`
}

type simpleCollocation struct {
	SearchMatch lemmaInfo `json:"searchMatch"`
	Collocate   lemmaInfo `json:"collocate"`
	Deprel      string    `json:"deprel"`
	LogDice     *float64  `json:"logDice"`
	TScore      *float64  `json:"tscore"`
	LMI         *float64  `json:"lmi"`
	LL          *float64  `json:"ll"`
	RRF         *float64  `json:"rrf"`
	MutualDist  *float64  `json:"mutualDist"`
}

type collResponse struct {
	Items []simpleCollocation `json:"items"`
	Error string              `json:"error"`
}
