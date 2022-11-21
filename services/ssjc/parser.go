// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package ssjc

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type dataStruct struct {
	Entry string `json:"entry"`
}

func parseData(src string) (*dataStruct, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	entry, err := doc.Find("div.entry").Html()
	if err != nil {
		return nil, err
	}
	ds := dataStruct{
		Entry: entry,
	}
	return &ds, nil
}
