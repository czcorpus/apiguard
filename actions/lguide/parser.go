// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/net/html"
)

type Span struct {
	value               string
	column, row         int
	columnSpan, rowSpan int
}

func getNextText(tkn *html.Tokenizer, endTag string) (text string) {
	isSup := false
	for {
		tt := tkn.Next()
		switch {
		case tt == html.ErrorToken:
			return
		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "sup" {
				isSup = true
			}
		case tt == html.TextToken:
			if !isSup {
				t := tkn.Token()
				text += t.Data
			}
		case tt == html.EndTagToken:
			t := tkn.Token()
			if endTag == "" || t.Data == endTag {
				return
			}
			if t.Data == "sup" {
				isSup = false
			}
		}
	}
}

func cleanUpSpanBuffer(spanBuffer []Span, row int) (newSpanBuffer []Span) {
	for _, span := range spanBuffer {
		if span.row+span.rowSpan-1 >= row {
			newSpanBuffer = append(newSpanBuffer, span)
		}
	}
	return
}

func parseTable(tkn *html.Tokenizer) (data map[string]string) {
	data = make(map[string]string)

	row, column := 0, 0
	var columns []string
	var rowName string
	var spanBuffer []Span

	for {
		if row > 0 && column > 0 {
			columnFilled := true
			for columnFilled {
				columnFilled = false
				for _, span := range spanBuffer {
					if span.column <= column && column < span.columnSpan+span.column {
						if span.columnSpan > 1 {
							if span.value != "" {
								data[rowName] = span.value
							}
							row++
							spanBuffer = cleanUpSpanBuffer(spanBuffer, row)
							column = 0
						} else {
							if span.value != "" {
								data[rowName+":"+columns[column]] = span.value
							}
							column++
							columnFilled = true
						}
						break
					}
				}
			}
		}

		tt := tkn.Next()
		switch {
		case tt == html.ErrorToken:
			return

		case tt == html.EndTagToken:
			t := tkn.Token()
			if t.Data == "table" {
				return
			} else if t.Data == "tr" {
				row++
				spanBuffer = cleanUpSpanBuffer(spanBuffer, row)
				column = 0
			}

		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "td" {
				text := getNextText(tkn, "")
				if row == 0 {
					columns = append(columns, text)
				} else if column == 0 {
					rowName = text
				} else {
					span := Span{value: text, column: column, row: row, columnSpan: 1, rowSpan: 1}
					for _, attr := range t.Attr {
						var err error = nil
						if attr.Key == "colspan" {
							span.columnSpan, err = strconv.Atoi(attr.Val)
						} else if attr.Key == "rowspan" {
							span.rowSpan, err = strconv.Atoi(attr.Val)
						}
						if err != nil {
							panic(err)
						}
					}

					// here expecting that colspan is always full table width
					if span.columnSpan > 1 {
						if text != "" {
							data[rowName] = text
						}
					} else {
						if text != "" {
							data[rowName+":"+columns[column]] = text
						}
					}

					if span.rowSpan > 1 {
						spanBuffer = append(spanBuffer, span)
					}
				}
				column++
			}
		}
	}
}

func Parse(text string) (data map[string]string) {
	data = make(map[string]string)
	tkn := html.NewTokenizer(strings.NewReader(text))

	for {
		tt := tkn.Next()

		switch {
		case tt == html.ErrorToken:
			return

		case tt == html.TextToken:
			t := tkn.Token()
			if strings.Contains(t.Data, "Heslové slovo bylo nalezeno také v následujících slovnících:") {
				return
			}

		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "table" {
				maps.Copy(data, parseTable(tkn))

			} else {
				for _, attr := range t.Attr {
					if attr.Key == "class" {
						if attr.Val == "hlavicka" {
							data["hlavička"] = getNextText(tkn, "")
							break

						} else if attr.Val == "polozky" {
							polozka := getNextText(tkn, t.Data)
							key_val := strings.SplitN(polozka, ": ", 2)
							data[key_val[0]] = key_val[1]
							break
						}
					}
				}
			}
		}
	}
}
