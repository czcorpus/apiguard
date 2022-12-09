// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

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
