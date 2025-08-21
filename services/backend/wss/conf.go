// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wss

import (
	"apiguard/guard"
	"apiguard/guard/token"
	"apiguard/services/cnc"
	"fmt"
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
