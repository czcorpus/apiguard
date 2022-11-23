// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kla

import (
	"math/rand"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseData(src string, maxItems int, baseURL string) ([]string, error) {
	images := make([]string, 0)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return images, err
	}
	nodes := doc.Find("table center img").Nodes
	for i := 0; i < maxItems; i++ {
		nodeCount := len(nodes)
		if nodeCount == 0 {
			break
		}
		v := rand.Intn(nodeCount)
		node := nodes[v]
		for _, attr := range node.Attr {
			if attr.Key == "src" {
				images = append(images, baseURL+"/"+attr.Val)
				break
			}
		}
		nodes = append(nodes[:v], nodes[v+1:]...)
	}
	return images, err
}
