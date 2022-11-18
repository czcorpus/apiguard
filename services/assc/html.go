// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

type exampleItem struct {
	Usage string   `json:"usage"`
	Data  []string `json:"data"`
}

type meaningItem struct {
	Explanation     string        `json:"explanation"`
	MetaExplanation string        `json:"metaExplanation"`
	Attachement     string        `json:"attachement"`
	Synonyms        []string      `json:"synonyms"`
	Examples        []exampleItem `json:"examples"`

	lastExample *exampleItem
}

func NewMeaningItem(def string) meaningItem {
	return meaningItem{
		Explanation:     def,
		MetaExplanation: "",
		Attachement:     "",
		Synonyms:        make([]string, 0),
		Examples:        make([]exampleItem, 0),
	}
}

type phrasemeItem struct {
	Phraseme    string   `json:"phraseme"`
	Explanation string   `json:"explanation"`
	Examples    []string `json:"examples"`
}

type collocationItem struct {
	Collocation string   `json:"collocation"`
	Explanation string   `json:"explanation"`
	Examples    []string `json:"examples"`
}

type dataItem struct {
	Key           string            `json:"key"`
	Pronunciation string            `json:"pronunciation"`
	AudioFile     string            `json:"audioFile"`
	Quality       string            `json:"quality"`
	Forms         map[string]string `json:"forms"`
	POS           string            `json:"pos"`
	Meaning       []meaningItem     `json:"meaning"`
	Phrasemes     []phrasemeItem    `json:"phrasemes"`
	Collocations  []collocationItem `json:"collocations"`

	lastMeaning     *meaningItem
	lastPhraseme    *phrasemeItem
	lastCollocation *collocationItem
}

func NewDataItem(heslo string) dataItem {
	return dataItem{
		Key:          heslo,
		Forms:        make(map[string]string),
		Meaning:      make([]meaningItem, 0),
		Phrasemes:    make([]phrasemeItem, 0),
		Collocations: make([]collocationItem, 0),
	}
}

type dataStruct struct {
	Items []dataItem `json:"items"`
	Notes []string   `json:"notes"`

	lastItem *dataItem
}

func (ds *dataStruct) AddItem(item dataItem) {
	ds.Items = append(ds.Items, item)
	ds.lastItem = &ds.Items[len(ds.Items)-1]
}

func NewDataStruct() dataStruct {
	return dataStruct{
		Items:    make([]dataItem, 0, 10),
		Notes:    make([]string, 0),
		lastItem: nil,
	}
}

func parseData(src string) (*dataStruct, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	subcont := doc.Find("div.subcont")
	ds := NewDataStruct()
	subcont.Children().Each(func(i int, s *goquery.Selection) {
		processNodes(s, &ds)
	})

	return &ds, err
}

func normalizeString(str string) string {
	return strings.Replace(strings.TrimSpace(strings.Trim(str, ": ")), "\u00a0", " ", -1)
}

func listContains(data []string, val string) bool {
	for _, v := range data {
		if v == val {
			return true
		}
	}
	return false
}

func processNodes(s *goquery.Selection, ds *dataStruct) {
	subsel := s.Find("span.heslo span.mainVar")
	if subsel.Length() > 0 {
		ds.AddItem(NewDataItem(normalizeString(subsel.Text())))
		return
	}

	if s.HasClass("vyslovnost") {
		src, ok := s.Find("span.mediPlayer audio source").Attr("src")
		if ok {
			ds.lastItem.AudioFile = src
		}

		data, err := s.Html()
		if err != nil {
			log.Warn().Msgf("Error when getting raw `vyslovnost` html: %s", err)
			return
		}
		tkn := html.NewTokenizer(strings.NewReader(data))
		text := ""

		for {
			tt := tkn.Next()
			switch {
			case tt == html.ErrorToken:
				ds.lastItem.Pronunciation = normalizeString(text)
				return

			case tt == html.TextToken:
				t := tkn.Token()
				text += t.Data

			case tt == html.StartTagToken:
				t := tkn.Token()
				switch t.Data {
				case "span":
					for _, attr := range t.Attr {
						if attr.Key == "class" && attr.Val == "mediPlayer" {
							ds.lastItem.Pronunciation = normalizeString(text)
							return
						}
					}
				}
			}
		}
	}

	if s.HasClass("stylKval") {
		ds.lastItem.Quality = normalizeString(s.Text())
		return
	}

	if s.HasClass("sl_druh") {
		ds.lastItem.POS = normalizeString(s.Text())
		return
	}

	if s.HasClass("ext_pozn_wrapper") {
		noteHTML, err := s.Find("span.ext_pozn").Html()
		if err != nil {
			log.Warn().Msgf("Error when getting raw `span.ext_pozn` html: %s", err)
		} else {
			ds.Notes = append(ds.Notes, noteHTML)
		}
		return
	}

	if s.HasClass("vyznam_wrapper") {
		if s.Find("span.vyz_count_num").Length() > 0 {
			meaning := NewMeaningItem(normalizeString(s.Find("span.vyznam").Text()))
			predvyklad := normalizeString(s.Find("span.predvyklad_wrap").Text())
			if len(predvyklad) > 0 {
				meaning.Explanation += " " + predvyklad
			}
			meaning.MetaExplanation = normalizeString(s.Find("span.metavyklad").Text())
			meaning.Attachement = normalizeString(s.Find("span.vazebnost").Text())
			s.Find("span.synonymum").Each(func(i int, s *goquery.Selection) {
				syn := normalizeString(s.Text())
				if syn != "" && !listContains(meaning.Synonyms, syn) {
					meaning.Synonyms = append(meaning.Synonyms, syn)
				}
			})
			ds.lastItem.Meaning = append(ds.lastItem.Meaning, meaning)
			ds.lastItem.lastMeaning = &ds.lastItem.Meaning[len(ds.lastItem.Meaning)-1]
			ds.lastItem.lastPhraseme = nil
			ds.lastItem.lastCollocation = nil
			ds.lastItem.lastMeaning.Examples = append(
				ds.lastItem.lastMeaning.Examples,
				exampleItem{
					Usage: "",
					Data:  make([]string, 0),
				},
			)
			ds.lastItem.lastMeaning.lastExample = &ds.lastItem.lastMeaning.Examples[len(ds.lastItem.lastMeaning.Examples)-1]
		} else if s.Find("span.vyz_count_bull").Length() > 0 {
			ds.lastItem.lastMeaning.Examples = append(
				ds.lastItem.lastMeaning.Examples,
				exampleItem{
					Usage: normalizeString(s.Children().Last().Text()),
					Data:  make([]string, 0),
				},
			)
			ds.lastItem.lastMeaning.lastExample = &ds.lastItem.lastMeaning.Examples[len(ds.lastItem.lastMeaning.Examples)-1]
		} else {
			log.Warn().Msgf("Unknown type of `vyznam_wrapper` element")
		}
		return
	}

	subsel = s.Find("span.frazem")
	if subsel.Length() > 0 {
		phraseme := phrasemeItem{
			Phraseme:    normalizeString(subsel.Text()),
			Explanation: "",
			Examples:    make([]string, 0),
		}
		ds.lastItem.Phrasemes = append(ds.lastItem.Phrasemes, phraseme)
		ds.lastItem.lastMeaning = nil
		ds.lastItem.lastPhraseme = &ds.lastItem.Phrasemes[len(ds.lastItem.Phrasemes)-1]
		ds.lastItem.lastCollocation = nil
		return
	}

	subsel = s.Find("span.souslovi")
	if subsel.Length() == 0 {
		subsel = s.Find("span.viceslovne")
	}
	if subsel.Length() > 0 {
		collocation := collocationItem{
			Collocation: normalizeString(subsel.Text()),
			Explanation: "",
			Examples:    make([]string, 0),
		}
		ds.lastItem.Collocations = append(ds.lastItem.Collocations, collocation)
		ds.lastItem.lastMeaning = nil
		ds.lastItem.lastPhraseme = nil
		ds.lastItem.lastCollocation = &ds.lastItem.Collocations[len(ds.lastItem.Collocations)-1]
		return
	}

	if s.HasClass("vyznam_wrapper_link") {
		if ds.lastItem.lastPhraseme != nil {
			ds.lastItem.lastPhraseme.Explanation = normalizeString(s.Find("span.vyznam").Text())
		} else if ds.lastItem.lastCollocation != nil {
			ds.lastItem.lastCollocation.Explanation = normalizeString(s.Find("span.vyznam").Text())
		} else {
			log.Warn().Msgf("Unknown `span.vyznam` parent: %s", s.Text())
		}
		return
	}

	// Here we have to use `html.Tokenizer`, because there is problematic document structure
	if s.HasClass("exeplifikace") {
		data, err := s.Html()
		if err != nil {
			log.Warn().Msgf("Error when getting raw `exeplifikace` html: %s", err)
			return
		}

		tkn := html.NewTokenizer(strings.NewReader(data))
		text := ""
		isNote := false

		for {
			tt := tkn.Next()
			switch {
			case tt == html.ErrorToken:
				if len(text) > 0 {
					if ds.lastItem.lastMeaning != nil {
						ds.lastItem.lastMeaning.lastExample.Data = append(ds.lastItem.lastMeaning.lastExample.Data, normalizeString(text))
					} else if ds.lastItem.lastPhraseme != nil {
						ds.lastItem.lastPhraseme.Examples = append(ds.lastItem.lastPhraseme.Examples, normalizeString(text))
					} else if ds.lastItem.lastCollocation != nil {
						ds.lastItem.lastCollocation.Examples = append(ds.lastItem.lastCollocation.Examples, normalizeString(text))
					} else {
						log.Warn().Msgf("Unknown `exeplifikace` parent: %s", text)
					}
				}
				return

			case tt == html.TextToken:
				t := tkn.Token()
				if isNote {
					if len(normalizeString(t.Data)) > 0 {
						text += "(" + normalizeString(t.Data) + ")"
					}
					isNote = false
				} else {
					text += t.Data
				}

			case tt == html.StartTagToken:
				t := tkn.Token()
				for _, attr := range t.Attr {
					if attr.Key == "class" && attr.Val == "small" {
						isNote = true
					}
				}

			case tt == html.SelfClosingTagToken:
				t := tkn.Token()
				if t.Data == "br" {
					if len(text) > 0 {
						if ds.lastItem.lastMeaning != nil {
							ds.lastItem.lastMeaning.lastExample.Data = append(ds.lastItem.lastMeaning.lastExample.Data, normalizeString(text))
						} else if ds.lastItem.lastPhraseme != nil {
							ds.lastItem.lastPhraseme.Examples = append(ds.lastItem.lastPhraseme.Examples, normalizeString(text))
						} else if ds.lastItem.lastCollocation != nil {
							ds.lastItem.lastCollocation.Examples = append(ds.lastItem.lastCollocation.Examples, normalizeString(text))
						} else {
							log.Warn().Msgf("Unknown `exeplifikace` parent: %s", text)
						}
						text = ""
					}
				}
			}
		}
	}

	// this has to be at the end, because more elements can contain `tvCh`
	// need to parse `vyznam_wrapper` first
	subsel = s.Find("span.tvCh")
	if subsel.Length() > 0 {
		var key string
		subsel.Children().Each(func(i int, s *goquery.Selection) {
			if s.HasClass("varianta-tvarChar") {
				key = normalizeString(s.Text())
			} else if s.HasClass("varianta-tvarChar-koncovka-tvar") {
				value, ok := ds.lastItem.Forms[key]
				if ok {
					ds.lastItem.Forms[key] = value + ", " + normalizeString(s.Text())
				} else {
					ds.lastItem.Forms[key] = normalizeString(s.Text())
				}
			}
		})
		return
	}
}
