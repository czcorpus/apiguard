// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

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

type mergeFreqsArgs struct {
	URLs []string `json:"urls"`
}

type FreqDistribItemList []*FreqDistribItem

type FreqDistribItem struct {
	Word string  `json:"word"`
	Freq int64   `json:"freq"`
	Base int64   `json:"base"`
	IPM  float32 `json:"ipm"`
}

type partialFreqResponse struct {
	ConcSize         int64              `json:"concSize"`
	CorpusSize       int64              `json:"corpusSize"`
	SubcSize         int64              `json:"subcSize,omitempty"`
	Freqs            []*FreqDistribItem `json:"freqs"`
	Fcrit            string             `json:"fcrit"`
	ExamplesQueryTpl string             `json:"examplesQueryTpl,omitempty"`
	Error            string             `json:"error,omitempty"`
}

type mergeFreqsResponse struct {
	Parts []*partialFreqResponse `json:"parts"`
	Error string                 `json:"error,omitempty"`
}

type speechesArgs struct {
	Corpname  string   `json:"corpname"`
	Subcorpus string   `json:"subcorpus"`
	Query     string   `json:"query"`
	Structs   []string `json:"structs"`
	LeftCtx   int      `json:"leftCtx"`
	RightCtx  int      `json:"rightCtx"`
}

func (c *Conf) Validate(context string) error {
	if err := c.ProxyConf.Validate(context); err != nil {
		return err
	}
	if c.GuardType == guard.GuardTypeToken && len(c.Tokens) == 0 {
		return fmt.Errorf("no tokens defined for token guard - the service won't be accessible")
	}
	return nil
}
