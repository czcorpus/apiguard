package main

import (
	"strings"

	"golang.org/x/net/html"
)

func parse(page string) (data [7][2]string) {

	tkn := html.NewTokenizer(strings.NewReader(page))

	var isData bool
	var isPlural bool = false
	var wordCase int = 0

	for {

		tt := tkn.Next()

		switch {

		case tt == html.ErrorToken:
			return data

		case tt == html.StartTagToken:

			t := tkn.Token()
			isData = t.Data == "x"

		case tt == html.TextToken:

			if isData {
				t := tkn.Token()

				if isPlural {
					data[wordCase][1] = t.Data
				} else {
					data[wordCase][0] = t.Data
				}

				isPlural = !isPlural
				if !isPlural {
					wordCase++
				}
			}

			isData = false
		}
	}
}
