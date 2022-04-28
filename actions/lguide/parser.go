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

type GrammarNumber struct {
	singular string
	plural   string
}

type GrammarCase struct {
	nominative   GrammarNumber
	genitive     GrammarNumber
	dative       GrammarNumber
	accusative   GrammarNumber
	vocative     GrammarNumber
	locative     GrammarNumber
	instrumental GrammarNumber
}

type GrammarPerson struct {
	first  GrammarNumber
	second GrammarNumber
	third  GrammarNumber
}

type Participle struct {
	active  string
	passive string
}

type TransgressiveRow struct {
	m  GrammarNumber
	zs GrammarNumber
}

type Transgressives struct {
	past    TransgressiveRow
	present TransgressiveRow
}

type VerbData struct {
	person         GrammarPerson
	imperative     GrammarNumber
	participle     Participle
	transgressives Transgressives
	verbalNoun     string
}

type ParsedData struct {
	heading     string
	items       map[string]string
	grammarCase GrammarCase
	verbData    VerbData
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
		word = &data.grammarCase.nominative
	case "2. pád":
		word = &data.grammarCase.genitive
	case "3. pád":
		word = &data.grammarCase.dative
	case "4. pád":
		word = &data.grammarCase.accusative
	case "5. pád":
		word = &data.grammarCase.vocative
	case "6. pád":
		word = &data.grammarCase.locative
	case "7. pád":
		word = &data.grammarCase.instrumental
	}

	if columnName == "jednotné číslo" {
		word.singular = value
	} else {
		word.plural = value
	}
}

func isVerbData(rowName string) bool {
	return strings.Contains(rowName, "osoba") || rowName == "rozkazovací způsob" || strings.Contains(rowName, "příčestí") || strings.Contains(rowName, "přechodník") || rowName == "verbální substantivum"
}

func (data *ParsedData) fillVerbData(rowName string, columnName string, value string) {
	var word *GrammarNumber

	switch rowName {
	case "1. osoba":
		word = &data.verbData.person.first
	case "2. osoba":
		word = &data.verbData.person.second
	case "3. osoba":
		word = &data.verbData.person.third
	case "rozkazovací způsob":
		word = &data.verbData.imperative
	case "příčestí činné":
		data.verbData.participle.active = value
	case "příčestí trpné":
		data.verbData.participle.passive = value
	case "přechodník přítomný, m.":
		word = &data.verbData.transgressives.present.m
	case "přechodník přítomný, ž. + s.":
		word = &data.verbData.transgressives.present.zs
	case "přechodník minulý, m.":
		word = &data.verbData.transgressives.past.m
	case "přechodník minulý, ž. + s.":
		word = &data.verbData.transgressives.past.zs
	case "verbální substantivum":
		data.verbData.verbalNoun = value
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

func Parse(text string) (data ParsedData) {
	data.items = make(map[string]string)
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
				data.parseTable(tkn)

			} else {
				for _, attr := range t.Attr {
					if attr.Key == "class" {
						if attr.Val == "hlavicka" {
							data.heading = getNextText(tkn, "")
							break

						} else if attr.Val == "polozky" {
							polozka := getNextText(tkn, t.Data)
							key_val := strings.SplitN(polozka, ": ", 2)
							data.items[key_val[0]] = key_val[1]
							break
						}
					}
				}
			}
		}
	}
}
