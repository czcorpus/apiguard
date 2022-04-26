// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/net/html"
)

func getNextText(tkn *html.Tokenizer) string {
	for {
		tt := tkn.Next()
		switch {
		case tt == html.EndTagToken:
			fallthrough
		case tt == html.ErrorToken:
			return ""
		case tt == html.TextToken:
			t := tkn.Token()
			return t.Data
		}
	}
}

func parseTable(tkn *html.Tokenizer) map[string]string {
	data := make(map[string]string)

	row, column := 0, 0
	var columns []string
	var rowName string

	for {
		tt := tkn.Next()
		switch {
		case tt == html.ErrorToken:
			return data

		case tt == html.EndTagToken:
			t := tkn.Token()
			if t.Data == "table" {
				return data
			} else if t.Data == "tr" {
				column = 0
				row++
			}

		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "td" {
				text := getNextText(tkn)
				if row == 0 {
					columns = append(columns, text)
				} else if column == 0 {
					rowName = text
				} else {
					hasColspan := false
					for _, attr := range t.Attr {
						// here expecting that colspan is always full table width
						if attr.Key == "colspan" {
							hasColspan = true
							break
						}
					}

					if hasColspan {
						data[rowName] = text
					} else {
						data[rowName+":"+columns[column]] = text
					}
				}
				column++
			}
		}
	}
}

func Parse(text string) map[string]string {
	data := make(map[string]string)
	tkn := html.NewTokenizer(strings.NewReader(text))

	for {
		tt := tkn.Next()

		switch {
		case tt == html.ErrorToken:
			return data

		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "table" {
				maps.Copy(data, parseTable(tkn))

			} else {
				for _, attr := range t.Attr {
					if attr.Key == "class" {
						if attr.Val == "hlavicka" {
							data["hlavička"] = getNextText(tkn)
							break

						} else if attr.Val == "polozky" {
							polozka := getNextText(tkn)
							key_val := strings.Split(polozka, ": ")
							data[key_val[0]] = key_val[1]
							break
						}
					}
				}
			}
		}
	}
}
