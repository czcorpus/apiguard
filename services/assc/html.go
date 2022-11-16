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

type meaningItem struct {
	Explanation     string   `json:"explanation"`
	MetaExplanation string   `json:"metaExplanation"`
	Attachement     string   `json:"attachement"`
	Synonyms        []string `json:"synonyms"`
	Examples        []string `json:"examples"`
}

func NewMeaningItem(def string) meaningItem {
	return meaningItem{
		Explanation:     def,
		MetaExplanation: "",
		Attachement:     "",
		Synonyms:        make([]string, 0),
		Examples:        make([]string, 0),
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
	Note          string            `json:"note"`

	lastMeaningItem     *meaningItem
	lastPhrasemeItem    *phrasemeItem
	lastCollocationItem *collocationItem
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
	lastItem *dataItem
	data     []dataItem
}

func (ds *dataStruct) AddItem(item dataItem) {
	ds.data = append(ds.data, item)
	ds.lastItem = &ds.data[len(ds.data)-1]
}

func NewDataStruct() dataStruct {
	return dataStruct{
		lastItem: nil,
		data:     make([]dataItem, 0, 10),
	}
}

func parseData(src string) ([]dataItem, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	subcont := doc.Find("div.subcont")
	ds := NewDataStruct()
	subcont.Children().Each(func(i int, s *goquery.Selection) {
		processNodes(s, &ds)
	})

	return ds.data, err
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

	if s.HasClass("ext_pozn_wrapper") {
		ds.lastItem.Note = normalizeString(s.Find("span.ext_pozn").Text())
		return
	}

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

	if s.HasClass("sl_druh") {
		ds.lastItem.POS = normalizeString(s.Text())
		return
	}

	if s.HasClass("vyznam_wrapper") {
		meaning := NewMeaningItem(normalizeString(s.Find("span.vyznam").Text()))
		meaning.MetaExplanation = normalizeString(s.Find("span.metavyklad").Text())
		meaning.Attachement = normalizeString(s.Find("span.vazebnost").Text())
		s.Find("span.synonymum").Each(func(i int, s *goquery.Selection) {
			syn := normalizeString(s.Find("span.synonymum").Text())
			if syn != "" && !listContains(meaning.Synonyms, syn) {
				meaning.Synonyms = append(meaning.Synonyms, syn)
			}
		})
		ds.lastItem.Meaning = append(ds.lastItem.Meaning, meaning)
		ds.lastItem.lastMeaningItem = &ds.lastItem.Meaning[len(ds.lastItem.Meaning)-1]
		ds.lastItem.lastPhrasemeItem = nil
		ds.lastItem.lastCollocationItem = nil
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
		ds.lastItem.lastMeaningItem = nil
		ds.lastItem.lastPhrasemeItem = &ds.lastItem.Phrasemes[len(ds.lastItem.Phrasemes)-1]
		ds.lastItem.lastCollocationItem = nil
		return
	}

	subsel = s.Find("span.souslovi")
	if subsel.Length() > 0 {
		collocation := collocationItem{
			Collocation: normalizeString(subsel.Text()),
			Explanation: "",
			Examples:    make([]string, 0),
		}
		ds.lastItem.Collocations = append(ds.lastItem.Collocations, collocation)
		ds.lastItem.lastMeaningItem = nil
		ds.lastItem.lastPhrasemeItem = nil
		ds.lastItem.lastCollocationItem = &ds.lastItem.Collocations[len(ds.lastItem.Collocations)-1]
		return
	}

	if s.HasClass("vyznam_wrapper_link") {
		if ds.lastItem.lastPhrasemeItem != nil {
			ds.lastItem.lastPhrasemeItem.Explanation = normalizeString(s.Find("span.vyznam").Text())
		} else if ds.lastItem.lastCollocationItem != nil {
			ds.lastItem.lastCollocationItem.Explanation = normalizeString(s.Find("span.vyznam").Text())
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
					if ds.lastItem.lastMeaningItem != nil {
						ds.lastItem.lastMeaningItem.Examples = append(ds.lastItem.lastMeaningItem.Examples, normalizeString(text))
					} else if ds.lastItem.lastPhrasemeItem != nil {
						ds.lastItem.lastPhrasemeItem.Examples = append(ds.lastItem.lastPhrasemeItem.Examples, normalizeString(text))
					} else if ds.lastItem.lastCollocationItem != nil {
						ds.lastItem.lastCollocationItem.Examples = append(ds.lastItem.lastCollocationItem.Examples, normalizeString(text))
					} else {
						log.Warn().Msgf("Unknown `exeplifikace` parent: %s", text)
					}
				}
				return

			case tt == html.TextToken:
				t := tkn.Token()
				if isNote {
					text += "(" + t.Data + ")"
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
						if ds.lastItem.lastMeaningItem != nil {
							ds.lastItem.lastMeaningItem.Examples = append(ds.lastItem.lastMeaningItem.Examples, normalizeString(text))
						} else if ds.lastItem.lastPhrasemeItem != nil {
							ds.lastItem.lastPhrasemeItem.Examples = append(ds.lastItem.lastPhrasemeItem.Examples, normalizeString(text))
						} else if ds.lastItem.lastCollocationItem != nil {
							ds.lastItem.lastCollocationItem.Examples = append(ds.lastItem.lastCollocationItem.Examples, normalizeString(text))
						} else {
							log.Warn().Msgf("Unknown `exeplifikace` parent: %s", text)
						}
						text = ""
					}
				}
			}
		}
	}
}
