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

package lguide

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Alternative struct {
	Id   string `json:"id"`
	Info string `json:"info"`
}

type ParsedData struct {
	Scripts         []string      `json:"scripts"`
	CSSLinks        []string      `json:"cssLinks"`
	Heading         string        `json:"heading"`
	Pronunciation   string        `json:"pronunciation"`
	Meaning         string        `json:"meaning"`
	Syllabification string        `json:"syllabification"`
	Gender          string        `json:"gender"`
	Conjugation     Conjugation   `json:"conjugation"`
	GrammarCase     GrammarCase   `json:"grammarCase"`
	Comparison      Comparison    `json:"comparison"`
	Examples        []string      `json:"examples"`
	Alternatives    []Alternative `json:"alternatives"`
	Notes           string        `json:"notes"`
	items           map[string]string
	Error           error
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
		word = &data.GrammarCase.Nominative
	case "2. pád":
		word = &data.GrammarCase.Genitive
	case "3. pád":
		word = &data.GrammarCase.Dative
	case "4. pád":
		word = &data.GrammarCase.Accusative
	case "5. pád":
		word = &data.GrammarCase.Vocative
	case "6. pád":
		word = &data.GrammarCase.Locative
	case "7. pád":
		word = &data.GrammarCase.Instrumental
	}

	if columnName == "jednotné číslo" {
		word.Singular = value
	} else {
		word.Plural = value
	}
}

func (data *ParsedData) fillComparison(key string, value string) {

	switch key {
	case "2. stupeň":
		data.Comparison.Comparative = value
	case "3. stupeň":
		data.Comparison.Superlative = value
	}
}

func isVerbData(rowName string) bool {
	return strings.Contains(rowName, "osoba") || rowName == "rozkazovací způsob" || strings.Contains(rowName, "příčestí") || strings.Contains(rowName, "přechodník") || rowName == "verbální substantivum"
}

func (data *ParsedData) fillVerbData(rowName string, columnName string, value string) {
	var word *GrammarNumber

	switch rowName {
	case "1. osoba":
		word = &data.Conjugation.Person.First
	case "2. osoba":
		word = &data.Conjugation.Person.Second
	case "3. osoba":
		word = &data.Conjugation.Person.Third
	case "rozkazovací způsob":
		word = &data.Conjugation.Imperative
	case "příčestí činné":
		data.Conjugation.Participle.Active = value
	case "příčestí trpné":
		data.Conjugation.Participle.Passive = value
	case "přechodník přítomný, m.":
		word = &data.Conjugation.Transgressive.Present.M
	case "přechodník přítomný, ž. + s.":
		word = &data.Conjugation.Transgressive.Present.ZS
	case "přechodník minulý, m.":
		word = &data.Conjugation.Transgressive.Past.M
	case "přechodník minulý, ž. + s.":
		word = &data.Conjugation.Transgressive.Past.ZS
	case "verbální substantivum":
		data.Conjugation.VerbalNoun = value
	default:
		panic("Unknown verb data!")
	}

	if word != nil {
		if columnName == "jednotné číslo" {
			word.Singular = value
		} else {
			word.Plural = value
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
		case tt == html.SelfClosingTagToken:
			t := tkn.Token()
			if t.Data == "td" {
				columns = append(columns, "")
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
							data.Error = err
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

func (data *ParsedData) parseAlternatives(tkn *html.Tokenizer) {
	var alt *Alternative
	for {
		tt := tkn.Next()
		switch {
		case tt == html.ErrorToken:
			return

		case tt == html.StartTagToken:
			t := tkn.Token()
			if t.Data == "a" {
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						id, err := url.QueryUnescape(strings.Split(attr.Val, "id=")[1])
						if err != nil {
							data.Error = err
						}
						alt = &Alternative{
							Id:   id,
							Info: "",
						}
						break
					}
				}
			}

		case tt == html.EndTagToken:
			t := tkn.Token()
			if t.Data == "a" {
				alt.Info = getNextText(tkn, "span")
				data.Alternatives = append(data.Alternatives, *alt)

			} else if t.Data == "div" {
				return
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
			} else if strings.Contains(t.Data, "Vyberte z nalezených hesel:") {
				data.parseAlternatives(tkn)
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
								data.Syllabification = key_val[1]
							} else if key_val[0] == "rod" {
								data.Gender = key_val[1]
							} else if key_val[0] == "příklady" {
								data.Examples = strings.Split(key_val[1], "; ")
							} else if key_val[0] == "poznámky k heslu" {
								data.Notes = key_val[1]
							} else if key_val[0] == "výslovnost" {
								data.Pronunciation = key_val[1]
							} else if key_val[0] == "význam" {
								data.Meaning = key_val[1]
							} else {
								if len(key_val) == 1 {
									data.Meaning = key_val[0]
								} else {
									data.items[key_val[0]] = key_val[1]
								}
							}
							break
						}
					}
				}
			}
		}
	}
}
