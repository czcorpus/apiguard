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

package ssjc

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
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
				log.Error().Err(err).Msg("")
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
