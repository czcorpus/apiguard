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
