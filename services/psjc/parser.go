// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package psjc

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func lookForSTI(src string) ([]int, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(`go\('(\d+)'\)`)
	if err != nil {
		return nil, err
	}
	STIs := make([]int, 0)
	doc.Find("span.hw.bo").Each(func(i int, s *goquery.Selection) {
		val, ok := s.Attr("onclick")
		if ok {
			value, err := strconv.Atoi(re.FindStringSubmatch(val)[1])
			if err != nil {
				return
			}
			STIs = append(STIs, value)
		}
	})
	if err != nil {
		return nil, err
	}
	return STIs, nil
}

func parseData(src string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return "", err
	}
	entry, err := doc.Find("div.entry").Html()
	if err != nil {
		return "", err
	}
	return entry, nil
}
