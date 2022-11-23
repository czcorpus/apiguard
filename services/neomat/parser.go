// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package neomat

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

func parseData(src string, maxItems int) ([]string, error) {
	entries := make([]string, 0)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return entries, err
	}
	doc.Find("table.search_zaznam").Each(func(i int, s *goquery.Selection) {
		if i >= maxItems {
			return
		}
		s.Find("tr.radek_1").Children().First().Remove()
		s.Find("tr.radek_2").Children().First().Remove()
		s.Find("tr.radek_3").Children().First().Remove()
		word := s.Find(".radek_1 a").Text()
		s.Find(".radek_1 a").ReplaceWithHtml(word)
		entry, err := s.Html()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get raw HTML")
			return
		}
		entries = append(entries, entry)
	})
	return entries, err
}
