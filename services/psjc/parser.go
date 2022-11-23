// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package psjc

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseData(src string) ([]string, error) {
	entries := make([]string, 0)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return entries, err
	}
	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		entry, err := s.Html()
		if err != nil {
			return
		}
		entries = append(entries, entry)
	})
	return entries, err
}
