// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type ParsedData struct {
	Scripts     []string    `json:"scripts"`
	CSSLinks    []string    `json:"cssLinks"`
	Heading     string      `json:"heading"`
	Division    string      `json:"division"`
	Conjugation Conjugation `json:"conjugation"`
	GrammarCase GrammarCase `json:"grammarCase"`
	Comparison  Comparison  `json:"comparison"`
	items       map[string]string
}

type TableCellSpan struct {
	value               string
	column, row         int
	columnSpan, rowSpan int
}

func cleanUpSpanBuffer(spanBuffer []TableCellSpan, row int) (newSpanBuffer []TableCellSpan) {
	for _, span := range spanBuffer {
		if span.row+span.rowSpan-1 >= row {
			newSpanBuffer = append(newSpanBuffer, span)
		}
	}
	return
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

func (data *ParsedData) fillCase(rowName string, columnName string, value string) {
	var word *GrammarNumber
	switch rowName {
	case "1. pád":
		word = &data.GrammarCase.nominative
	case "2. pád":
		word = &data.GrammarCase.genitive
	case "3. pád":
		word = &data.GrammarCase.dative
	case "4. pád":
		word = &data.GrammarCase.accusative
	case "5. pád":
		word = &data.GrammarCase.vocative
	case "6. pád":
		word = &data.GrammarCase.locative
	case "7. pád":
		word = &data.GrammarCase.instrumental
	}

	if columnName == "jednotné číslo" {
		word.singular = value
	} else {
		word.plural = value
	}
}

func (data *ParsedData) fillComparison(key string, value string) {

	switch key {
	case "2. stupeň":
		data.Comparison.comparative = value
	case "3. stupeň":
		data.Comparison.superlative = value
	}
}

func isVerbData(rowName string) bool {
	return strings.Contains(rowName, "osoba") || rowName == "rozkazovací způsob" || strings.Contains(rowName, "příčestí") || strings.Contains(rowName, "přechodník") || rowName == "verbální substantivum"
}

func (data *ParsedData) fillVerbData(rowName string, columnName string, value string) {
	var word *GrammarNumber

	switch rowName {
	case "1. osoba":
		word = &data.Conjugation.person.first
	case "2. osoba":
		word = &data.Conjugation.person.second
	case "3. osoba":
		word = &data.Conjugation.person.third
	case "rozkazovací způsob":
		word = &data.Conjugation.imperative
	case "příčestí činné":
		data.Conjugation.participle.active = value
	case "příčestí trpné":
		data.Conjugation.participle.passive = value
	case "přechodník přítomný, m.":
		word = &data.Conjugation.transgressive.present.m
	case "přechodník přítomný, ž. + s.":
		word = &data.Conjugation.transgressive.present.zs
	case "přechodník minulý, m.":
		word = &data.Conjugation.transgressive.past.m
	case "přechodník minulý, ž. + s.":
		word = &data.Conjugation.transgressive.past.zs
	case "verbální substantivum":
		data.Conjugation.verbalNoun = value
	default:
		panic("Unknown verb data!")
	}

	if word != nil {
		if columnName == "jednotné číslo" {
			word.singular = value
		} else {
			word.plural = value
		}
	}
}

func (data *ParsedData) parseTable(tkn *html.Tokenizer) {
	row, column := 0, 0
	var columns []string
	var rowName string
	var spanBuffer []TableCellSpan

	for {
		if row > 0 && column > 0 {
			columnFilled := true
			for columnFilled {
				columnFilled = false
				for _, span := range spanBuffer {
					if span.column <= column && column < span.columnSpan+span.column {
						if span.columnSpan > 1 {
							if span.value != "" {
								if isVerbData(rowName) {
									data.fillVerbData(rowName, "", span.value)
								} else {
									data.items[rowName] = span.value
								}
							}
							row++
							spanBuffer = cleanUpSpanBuffer(spanBuffer, row)
							column = 0
						} else {
							if span.value != "" {
								if strings.Contains(rowName, "pád") {
									data.fillCase(rowName, columns[column], span.value)
								} else if isVerbData(rowName) {
									data.fillVerbData(rowName, columns[column], span.value)
								} else {
									data.items[rowName+":"+columns[column]] = span.value
								}
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
					span := TableCellSpan{value: text, column: column, row: row, columnSpan: 1, rowSpan: 1}
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
							if isVerbData(rowName) {
								data.fillVerbData(rowName, "", text)
							} else {
								data.items[rowName] = text
							}
						}
					} else {
						if text != "" {
							if strings.Contains(rowName, "pád") {
								data.fillCase(rowName, columns[column], span.value)
							} else if isVerbData(rowName) {
								data.fillVerbData(rowName, columns[column], span.value)
							} else {
								data.items[rowName+":"+columns[column]] = span.value
							}
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

func NewParsedData() *ParsedData {
	return &ParsedData{
		Scripts:  make([]string, 0, 10),
		CSSLinks: make([]string, 0, 10),
	}
}

func Parse(text string) *ParsedData {
	data := NewParsedData()
	data.items = make(map[string]string)
	tkn := html.NewTokenizer(strings.NewReader(text))

	for {
		tt := tkn.Next()

		switch {
		case tt == html.ErrorToken:
			return data

		case tt == html.TextToken:
			t := tkn.Token()
			if strings.Contains(t.Data, "Heslové slovo bylo nalezeno také v následujících slovnících:") {
				return data
			}

		case tt == html.StartTagToken:
			t := tkn.Token()
			switch t.Data {
			case "table":
				data.parseTable(tkn)
			case "script":
				for _, attr := range t.Attr {
					if attr.Key == "src" {
						data.Scripts = append(data.Scripts, attr.Val)
						break
					}
				}
			case "link":
				for _, attr := range t.Attr {
					if attr.Key == "href" && strings.HasSuffix(attr.Val, ".css") {
						data.CSSLinks = append(data.CSSLinks, attr.Val)
						break
					}
				}
			default:
				for _, attr := range t.Attr {
					if attr.Key == "class" {
						if attr.Val == "hlavicka" {
							data.Heading = getNextText(tkn, "")
							break

						} else if attr.Val == "polozky" {
							polozka := getNextText(tkn, t.Data)
							key_val := strings.SplitN(polozka, ": ", 2)
							if strings.Contains(key_val[0], "stupeň") {
								data.fillComparison(key_val[0], key_val[1])
							} else if key_val[0] == "dělení" {
								data.Division = key_val[1]
							} else {
								data.items[key_val[0]] = key_val[1]
							}
							break
						}
					}
				}
			}
		}
	}
}
