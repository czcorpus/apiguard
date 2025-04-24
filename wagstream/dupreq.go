// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

// groupedRequests maps between tile URLs (here keys)
// and tileIDs (here values)
type groupedRequests map[int][]*request

// register adds a new tileID=>URL pair.
func (ureqs groupedRequests) register(req *request) {
	reqKey := req.groupingKey()
	vals, ok := ureqs[reqKey]
	if ok {
		vals = append(vals, req)
		ureqs[reqKey] = vals

	} else {
		ureqs[reqKey] = make([]*request, 1, 10)
		ureqs[reqKey][0] = req
	}
}

func (ureqs groupedRequests) String() string {
	var ans strings.Builder
	var i int
	for k, v := range ureqs {
		values := strings.Join(
			collections.SliceMap(v, func(v *request, i int) string {
				return strconv.Itoa(v.TileID)
			}),
			", ",
		)
		if i > 0 {
			ans.WriteString("; ")
		}
		ans.WriteString(fmt.Sprintf("%d => %s", k, values))
		i++
	}
	return fmt.Sprintf("groupedRequests{ %s }", ans.String())
}

func (ureqs groupedRequests) valIter(yield func(item *request, tiles []int) bool) {
	for _, v := range ureqs {
		tiles := make([]int, len(v))
		for i, t := range v {
			tiles[i] = t.TileID
		}
		if !yield(v[0], tiles) {
			return
		}
	}
}
