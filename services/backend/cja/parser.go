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

package cja

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseData(src string, baseURL string) (*Response, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	response := Response{}

	cssLink := doc.Find(`link[rel="stylesheet"]`)
	cssHref, exists := cssLink.Attr("href")
	if exists {
		response.CSS = baseURL + cssHref
	}

	article := doc.Find("div.container")
	article.Find(`div[style="width: 640px"]`).RemoveAttr("style")
	article.Find("a").Remove()
	article.Find("script").Remove()
	img := article.Find("img").Remove()
	imgSRC, exists := img.Attr("src")
	if exists {
		response.Image = baseURL + imgSRC
	}

	response.Content, err = article.Html()
	if err != nil {
		return nil, err
	}

	return &response, err
}
