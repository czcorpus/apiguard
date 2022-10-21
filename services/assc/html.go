// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type meaningItem struct {
	Explanation     string   `json:"explanation"`
	MetaExplanation string   `json:"metaExplanation"`
	Examples        []string `json:"examples"`
}

func NewMeaningItem(def string) meaningItem {
	return meaningItem{
		Explanation:     def,
		MetaExplanation: "",
		Examples:        make([]string, 0),
	}
}

type dataItem struct {
	Key           string            `json:"key"`
	Pronunciation string            `json:"pronunciation"`
	Quality       string            `json:"quality"`
	Forms         map[string]string `json:"forms"`
	POS           string            `json:"pos"`
	Meaning       []meaningItem     `json:"meaning"`
	Note          string            `json:"note"`

	lastMeaningItem *meaningItem
}

func NewDataItem(heslo string) dataItem {
	return dataItem{
		Key:     heslo,
		Forms:   make(map[string]string),
		Meaning: make([]meaningItem, 0),
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

func normalizeHTML(src string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return "", err
	}
	subcont := doc.Find("div.subcont")
	ds := NewDataStruct()
	subcont.Children().Each(func(i int, s *goquery.Selection) {
		processNodes(s, &ds)
	})

	js, err := json.Marshal(ds.data)
	return string(js), err
}

func processNodes(s *goquery.Selection, ds *dataStruct) {
	subsel := s.Find("span.heslo span.mainVar")
	if subsel.Length() > 0 {
		ds.AddItem(NewDataItem(subsel.Text()))
		return
	}

	if s.HasClass("vyslovnost") {
		ds.lastItem.Pronunciation = s.Text()
		return
	}

	if s.HasClass("stylKval") {
		ds.lastItem.Quality = s.Text()
		return
	}

	if s.HasClass("ext_pozn_wrapper") {
		ds.lastItem.Note = s.Find("span.ext_pozn").Text()
		return
	}

	subsel = s.Find("span.tvCh")
	if subsel.Length() > 0 {
		var key string
		subsel.Children().Each(func(i int, s *goquery.Selection) {
			if s.HasClass("varianta-tvarChar") {
				key = s.Text()
			} else if s.HasClass("varianta-tvarChar-koncovka-tvar") {
				value, ok := ds.lastItem.Forms[key]
				if ok {
					ds.lastItem.Forms[key] = value + ", " + s.Text()
				} else {
					ds.lastItem.Forms[key] = s.Text()
				}
			}
		})
		return
	}

	if s.HasClass("sl_druh") {
		ds.lastItem.POS = s.Text()
		return
	}

	if s.HasClass("vyznam_wrapper") {
		meaning := NewMeaningItem(s.Find("span.vyznam").Text())
		meaning.MetaExplanation = s.Find("span.metavyklad").Text()
		ds.lastItem.Meaning = append(ds.lastItem.Meaning, meaning)
		ds.lastItem.lastMeaningItem = &ds.lastItem.Meaning[len(ds.lastItem.Meaning)-1]
		return
	}

	// TODO, there is problematic document structure
	if s.HasClass("exeplifikace") {
		s.First().Children().Each(func(i int, s *goquery.Selection) {
			if len(s.Text()) > 0 {
				ds.lastItem.lastMeaningItem.Examples = append(ds.lastItem.lastMeaningItem.Examples, s.Text())
			}
		})
		return
	}
}
