// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mquery

import (
	"fmt"
	"net/url"
)

type concordanceRequestArgs struct {
	Q             string
	Subcorpus     string
	Format        string
	ShowMarkup    bool
	ShowTextProps bool
	ContextWidth  int
	Coll          string
	CollRange     string
}

func (cra *concordanceRequestArgs) ToURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("q", cra.Q)
	if cra.Subcorpus != "" {
		q.Add("subcorpus", cra.Subcorpus)
	}
	if cra.Format != "" {
		q.Add("format", cra.Format)
	}
	if cra.ShowMarkup {
		q.Add("showMarkup", "1")
	}
	if cra.ShowTextProps {
		q.Add("showTextProps", "1")
	}
	q.Add("contextWidth", fmt.Sprint(cra.ContextWidth))
	if cra.Coll != "" {
		q.Add("coll", cra.Coll)
	}
	if cra.CollRange != "" {
		q.Add("collRange", cra.CollRange)
	}
	return q.Encode()
}

type tokenContextRequestArgs struct {
	Pos      int
	LeftCtx  int
	RightCtx int
	Attrs    []string
	Structs  []string
}

func (tcra *tokenContextRequestArgs) ToURLQuery() string {
	u := url.URL{}
	q := u.Query()
	q.Add("idx", fmt.Sprint(tcra.Pos))
	q.Add("leftCtx", fmt.Sprint(tcra.LeftCtx))
	q.Add("rightCtx", fmt.Sprint(tcra.RightCtx))
	for _, attr := range tcra.Attrs {
		q.Add("attr", attr)
	}
	for _, struc := range tcra.Structs {
		q.Add("struct", struc)
	}
	return q.Encode()
}

type concordanceToken struct {
	Type   string            `json:"type"`
	Word   string            `json:"word"`
	Strong bool              `json:"strong"`
	Attrs  map[string]string `json:"attrs"`
}

type concordanceLine struct {
	Text []concordanceToken `json:"text"`
	Ref  string             `json:"ref"`
}

type concordanceResponse struct {
	Lines      []concordanceLine `json:"lines"`
	ConcSize   int               `json:"concSize"`
	ResultType string            `json:"resultType"`
	Error      string            `json:"error,omitempty"`
}

type tokenContextResponse struct {
	Context    concordanceLine `json:"context"`
	ResultType string          `json:"resultType"`
	Error      string          `json:"error,omitempty"`
}
