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

package psjc

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
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
			log.Error().Err(err).Msg("")
			return
		}
		entries = append(entries, entry)
	})
	return entries, err
}
